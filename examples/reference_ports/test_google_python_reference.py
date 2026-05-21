import os
import sys
import time

REFERENCE_ROOT = r"D:\golang\go-camoufox\references\camoufox\pythonlib"
if REFERENCE_ROOT not in sys.path:
    sys.path.insert(0, REFERENCE_ROOT)

from camoufox.sync_api import Camoufox  # noqa: E402


def main() -> None:
    executable_path = os.environ.get("CAMOUFOX_EXECUTABLE")
    if not executable_path:
        raise SystemExit("set CAMOUFOX_EXECUTABLE to the Camoufox browser executable path")
    headless = os.environ.get("CAMOUFOX_HEADLESS", "").lower() in {"1", "true", "yes"}
    auto_close_seconds = float(os.environ.get("CAMOUFOX_AUTO_CLOSE_SECONDS", "0") or "0")

    with Camoufox(headless=headless, executable_path=executable_path) as browser:
        page = browser.new_page()

        page.goto("https://www.google.com")
        page.wait_for_load_state("networkidle", timeout=30_000)

        title = page.title()
        print(f"Page title: {title}")

        search_box = page.query_selector("textarea[name='q'], input[name='q']")
        if search_box:
            label = search_box.get_attribute("aria-label") or "(empty)"
            print(f"Search box label: {label}")

            query = "go-camoufox github"
            search_box.click()
            search_box.type(query, delay=120)
            search_box.press("Enter")

            page.wait_for_load_state("networkidle", timeout=30_000)
            print(f"Results title: {page.title()}")
            print(f"Current URL: {page.url}")
            challenge = page.evaluate(
                """
                (() => {
                  const text = document.body ? document.body.innerText : "";
                  return /unusual traffic|not a robot|captcha/i.test(text);
                })()
                """
            )
            print(f"Possible challenge detected: {challenge}")
        else:
            print("Search box not found - page layout may differ in this region.")

        ua = page.evaluate("navigator.userAgent")
        print(f"User-Agent: {ua}")
        metrics = page.evaluate(
            """
            () => ({
              outerWidth: window.outerWidth,
              outerHeight: window.outerHeight,
              innerWidth: window.innerWidth,
              innerHeight: window.innerHeight,
              screenWidth: screen.width,
              screenHeight: screen.height,
              availWidth: screen.availWidth,
              availHeight: screen.availHeight,
            })
            """
        )
        print(f"Window metrics: {metrics}")

        if auto_close_seconds > 0:
            print(f"\nAuto-closing in {auto_close_seconds} seconds...")
            time.sleep(auto_close_seconds)
        else:
            input("\nPress Enter to close the browser...")


if __name__ == "__main__":
    main()
