import java.net.URI;
import org.openqa.selenium.WebDriver;
import org.openqa.selenium.firefox.FirefoxOptions;
import org.openqa.selenium.remote.RemoteWebDriver;

class CamoufoxBidiExample {
  public static void main(String[] args) throws Exception {
    String rawEndpoint = System.getenv("CAMOUFOX_BIDI_ENDPOINT");
    if (rawEndpoint == null || rawEndpoint.isEmpty()) {
      throw new IllegalStateException(
          "set CAMOUFOX_BIDI_ENDPOINT, for example ws://127.0.0.1:50123/session");
    }
    URI endpoint = URI.create(rawEndpoint);
    WebDriver driver = new RemoteWebDriver(endpoint.toURL(), new FirefoxOptions());
    try {
      driver.get("https://example.com");
      System.out.println(driver.getTitle());
    } finally {
      driver.quit();
    }
  }
}
