import importlib.util
import json
import random
import sys
import types
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
REF = ROOT / "references" / "camoufox" / "pythonlib" / "camoufox"


def _install_shims() -> None:
    browserforge = types.ModuleType("browserforge")
    browserforge_fingerprints = types.ModuleType("browserforge.fingerprints")

    class Fingerprint:
        pass

    class FingerprintGenerator:
        def __init__(self, *args, **kwargs):
            pass

        def generate(self, **kwargs):
            raise RuntimeError("BrowserForge generation is not used by this parity harness")

    class ScreenFingerprint:
        pass

    browserforge_fingerprints.Fingerprint = Fingerprint
    browserforge_fingerprints.FingerprintGenerator = FingerprintGenerator
    browserforge_fingerprints.ScreenFingerprint = ScreenFingerprint
    sys.modules["browserforge"] = browserforge
    sys.modules["browserforge.fingerprints"] = browserforge_fingerprints

    camoufox = types.ModuleType("camoufox")
    camoufox.__path__ = [str(REF)]
    sys.modules["camoufox"] = camoufox

    pkgman = types.ModuleType("camoufox.pkgman")

    def load_yaml(name):
        if name != "browserforge.yml":
            raise RuntimeError(f"unsupported yaml fixture: {name}")
        # Tiny parser for the pinned browserforge.yml mapping. It intentionally
        # handles only the nested string-map shape used by that asset.
        root = {}
        stack = [(-1, root)]
        with open(REF / name, "r", encoding="utf-8") as fh:
            for raw in fh:
                if not raw.strip() or raw.lstrip().startswith("#"):
                    continue
                indent = len(raw) - len(raw.lstrip(" "))
                line = raw.strip()
                if "#" in line:
                    line = line.split("#", 1)[0].rstrip()
                if not line:
                    continue
                while stack and indent <= stack[-1][0]:
                    stack.pop()
                parent = stack[-1][1]
                key, _, value = line.partition(":")
                key = key.strip()
                value = value.strip()
                if value:
                    parent[key] = value
                else:
                    child = {}
                    parent[key] = child
                    stack.append((indent, child))
        return root

    pkgman.load_yaml = load_yaml
    pkgman.LOCAL_DATA = REF
    sys.modules["camoufox.pkgman"] = pkgman

    warnings = types.ModuleType("camoufox._warnings")

    class LeakWarning(RuntimeWarning):
        @staticmethod
        def warn(*args, **kwargs):
            return None

    warnings.LeakWarning = LeakWarning
    sys.modules["camoufox._warnings"] = warnings

    numpy = types.ModuleType("numpy")
    numpy.unique = lambda values: list(dict.fromkeys(values))
    numpy.ndarray = list
    sys.modules["numpy"] = numpy

    language_tags = types.ModuleType("language_tags")

    class _Part:
        def __init__(self, value):
            self.data = {"record": {"Subtag": value}}

    class _Tag:
        def __init__(self, value):
            parts = value.split("-")
            self.language = _Part(parts[0])
            self.region = _Part(parts[-1]) if len(parts) > 1 else None

    class _Tags:
        @staticmethod
        def check(value):
            return isinstance(value, str) and bool(value)

        @staticmethod
        def tag(value):
            return _Tag(value)

    language_tags.tags = _Tags()
    sys.modules["language_tags"] = language_tags

    webgl = types.ModuleType("camoufox.webgl")
    webgl.sample_webgl = lambda *args, **kwargs: {
        "webGl:vendor": "Shim Vendor",
        "webGl:renderer": "Shim Renderer",
        "webGl2Enabled": True,
    }
    sys.modules["camoufox.webgl"] = webgl

    orjson = types.ModuleType("orjson")
    orjson.loads = lambda data: json.loads(data.decode("utf-8") if isinstance(data, bytes) else data)
    orjson.dumps = lambda value: json.dumps(value, separators=(",", ":")).encode("utf-8")
    sys.modules["orjson"] = orjson


def _load_fingerprints():
    _install_shims()
    spec = importlib.util.spec_from_file_location("camoufox.fingerprints", REF / "fingerprints.py")
    module = importlib.util.module_from_spec(spec)
    sys.modules["camoufox.fingerprints"] = module
    spec.loader.exec_module(module)
    return module


def main() -> None:
    fp = _load_fingerprints()
    command = sys.argv[1]

    if command == "preset_counts":
        out = {
            "148": sum(len(v) for v in fp.load_presets(148)["presets"].values()),
            "149": sum(len(v) for v in fp.load_presets(149)["presets"].values()),
            "150": sum(len(v) for v in fp.load_presets(150)["presets"].values()),
        }
    elif command == "from_preset":
        random.seed(7)
        preset = json.loads(sys.stdin.read())
        out = fp.from_preset(preset, "151")
    elif command == "font_subset":
        random.seed(7)
        out = {
            "macos": fp._generate_random_font_subset("macos"),
            "linux": fp._generate_random_font_subset("linux"),
            "windows": fp._generate_random_font_subset("windows"),
        }
    elif command == "voice_subset":
        random.seed(7)
        out = {
            "macos": fp._generate_random_voice_subset("macos"),
            "linux": fp._generate_random_voice_subset("linux"),
            "windows": fp._generate_random_voice_subset("windows"),
        }
    elif command == "init_script":
        values = json.loads(sys.stdin.read())
        out = fp._build_init_script(values)
    elif command == "context_fingerprint_from_preset":
        random.seed(7)
        data = json.loads(sys.stdin.read())
        out = fp.generate_context_fingerprint(
            preset=data["preset"],
            ff_version=data.get("ff_version"),
            webrtc_ip=data.get("webrtc_ip"),
            timezone=data.get("timezone"),
            locale=data.get("locale"),
            config_overrides=data.get("config_overrides"),
        )
        out = {
            "context_options": out["context_options"],
            "config": out["config"],
            "init_script": out["init_script"],
        }
    else:
        raise SystemExit(f"unknown command: {command}")

    print(json.dumps(out, sort_keys=True, separators=(",", ":")))


if __name__ == "__main__":
    main()
