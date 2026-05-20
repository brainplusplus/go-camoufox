import os

from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.firefox.options import Options


endpoint = os.environ.get("CAMOUFOX_BIDI_ENDPOINT")
if not endpoint:
    raise SystemExit("set CAMOUFOX_BIDI_ENDPOINT, for example ws://127.0.0.1:50123/session")

options = Options()
driver = webdriver.Remote(command_executor=endpoint, options=options)
try:
    driver.get("https://example.com")
    print(driver.find_element(By.TAG_NAME, "h1").text)
finally:
    driver.quit()
