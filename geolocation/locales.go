package geolocation

import (
	"encoding/xml"
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/brainplusplus/go-camoufox/internal/assets"
)

type Locale struct {
	Language string
	Region   string
	Script   string
}

func (l Locale) String() string {
	if l.Region != "" {
		return l.Language + "-" + l.Region
	}
	return l.Language
}

func (l Locale) AsConfig() map[string]string {
	out := map[string]string{
		"locale:language": l.Language,
		"locale:region":   l.Region,
	}
	if l.Script != "" {
		out["locale:script"] = l.Script
	}
	return out
}

type Geolocation struct {
	Locale    Locale
	Longitude float64
	Latitude  float64
	Timezone  string
	Accuracy  *float64
}

func (g Geolocation) AsConfig() map[string]any {
	out := map[string]any{
		"geolocation:longitude": g.Longitude,
		"geolocation:latitude":  g.Latitude,
		"timezone":              g.Timezone,
	}
	for key, value := range g.Locale.AsConfig() {
		out[key] = value
	}
	if g.Accuracy != nil {
		out["geolocation:accuracy"] = *g.Accuracy
	}
	return out
}

var localeTagRe = regexp.MustCompile(`^[A-Za-z]{2,3}(?:-[A-Za-z]{4})?(?:-[A-Za-z]{2}|-[0-9]{3})?$`)

func NormalizeLocale(value string) (Locale, error) {
	value = strings.TrimSpace(value)
	if !localeTagRe.MatchString(value) {
		return Locale{}, invalidLocale(value)
	}
	parts := strings.Split(value, "-")
	if len(parts) < 2 {
		return Locale{}, invalidLocale(value)
	}
	locale := Locale{Language: strings.ToLower(parts[0])}
	if len(parts) == 3 {
		locale.Script = titleCase(parts[1])
		locale.Region = strings.ToUpper(parts[2])
	} else {
		locale.Region = strings.ToUpper(parts[1])
	}
	return locale, nil
}

func HandleLocale(value string, ignoreRegion bool) (Locale, error) {
	value = strings.TrimSpace(value)
	if len(value) > 3 {
		return NormalizeLocale(value)
	}
	selector, err := defaultSelector()
	if err != nil {
		return Locale{}, err
	}
	if locale, err := selector.FromRegion(strings.ToUpper(value)); err == nil {
		return locale, nil
	}
	if ignoreRegion {
		if !regexp.MustCompile(`^[A-Za-z]{2,3}$`).MatchString(value) {
			return Locale{}, invalidLocale(value)
		}
		return Locale{Language: strings.ToLower(value)}, nil
	}
	if locale, err := selector.FromLanguage(strings.ToLower(value)); err == nil {
		return locale, nil
	}
	return Locale{}, invalidLocale(value)
}

func HandleLocales(locales []string, config map[string]any) error {
	if len(locales) == 0 {
		return nil
	}
	first, err := HandleLocale(locales[0], false)
	if err != nil {
		return err
	}
	for key, value := range first.AsConfig() {
		config[key] = value
	}
	if len(locales) == 1 {
		return nil
	}
	all := make([]string, 0, len(locales))
	seen := map[string]struct{}{}
	for _, item := range locales {
		locale, err := HandleLocale(item, true)
		if err != nil {
			return err
		}
		text := locale.String()
		if _, ok := seen[text]; !ok {
			all = append(all, text)
			seen[text] = struct{}{}
		}
	}
	config["locale:all"] = strings.Join(all, ", ")
	return nil
}

type StatisticalLocaleSelector struct {
	territories map[string]territoryData
}

type territoryInfoXML struct {
	Territories []territoryXML `xml:"territory"`
}

type territoryXML struct {
	Type                string                  `xml:"type,attr"`
	Population          float64                 `xml:"population,attr"`
	LiteracyPercent     float64                 `xml:"literacyPercent,attr"`
	LanguagePopulations []languagePopulationXML `xml:"languagePopulation"`
}

type languagePopulationXML struct {
	Type              string  `xml:"type,attr"`
	PopulationPercent float64 `xml:"populationPercent,attr"`
}

type territoryData struct {
	Population      float64
	LiteracyPercent float64
	Languages       []weightedString
}

type weightedString struct {
	Value  string
	Weight float64
}

var (
	selectorOnce sync.Once
	selector     *StatisticalLocaleSelector
	selectorErr  error
)

func defaultSelector() (*StatisticalLocaleSelector, error) {
	selectorOnce.Do(func() {
		selector, selectorErr = LoadLocaleSelector()
	})
	return selector, selectorErr
}

func LoadLocaleSelector() (*StatisticalLocaleSelector, error) {
	data, err := assets.ReadFile("territoryInfo.xml")
	if err != nil {
		return nil, err
	}
	var parsed territoryInfoXML
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	territories := make(map[string]territoryData, len(parsed.Territories))
	for _, territory := range parsed.Territories {
		item := territoryData{Population: territory.Population, LiteracyPercent: territory.LiteracyPercent}
		for _, lang := range territory.LanguagePopulations {
			if lang.Type == "" || lang.PopulationPercent <= 0 {
				continue
			}
			item.Languages = append(item.Languages, weightedString{
				Value:  strings.ReplaceAll(lang.Type, "_", "-"),
				Weight: lang.PopulationPercent,
			})
		}
		if len(item.Languages) > 0 {
			territories[territory.Type] = item
		}
	}
	return &StatisticalLocaleSelector{territories: territories}, nil
}

func (s *StatisticalLocaleSelector) FromRegion(region string) (Locale, error) {
	region = strings.ToUpper(region)
	territory, ok := s.territories[region]
	if !ok {
		return Locale{}, fmt.Errorf("unknown territory: %s", region)
	}
	language := weightedChoice(territory.Languages)
	return NormalizeLocale(language + "-" + region)
}

func (s *StatisticalLocaleSelector) FromLanguage(language string) (Locale, error) {
	language = strings.ToLower(language)
	candidates := make([]weightedString, 0)
	for region, territory := range s.territories {
		for _, lang := range territory.Languages {
			base := strings.Split(lang.Value, "-")[0]
			if base != language {
				continue
			}
			weight := lang.Weight * territory.LiteracyPercent / 10000 * territory.Population
			if weight > 0 {
				candidates = append(candidates, weightedString{Value: region, Weight: weight})
			}
		}
	}
	if len(candidates) == 0 {
		return Locale{}, fmt.Errorf("unknown language: %s", language)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Value < candidates[j].Value })
	region := weightedChoice(candidates)
	return NormalizeLocale(language + "-" + region)
}

func weightedChoice(values []weightedString) string {
	if len(values) == 0 {
		return ""
	}
	total := 0.0
	for _, item := range values {
		total += item.Weight
	}
	if total <= 0 {
		return values[0].Value
	}
	needle := rand.Float64() * total
	for _, item := range values {
		needle -= item.Weight
		if needle <= 0 {
			return item.Value
		}
	}
	return values[len(values)-1].Value
}

func titleCase(value string) string {
	value = strings.ToLower(value)
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func invalidLocale(value string) error {
	return fmt.Errorf("invalid locale: %q. Must be either a region, language, language-region, or language-script-region", value)
}
