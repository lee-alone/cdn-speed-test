import logging
from gui import CloudflareSpeedTest
import tkinter as tk

logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s")
logger = logging.getLogger(__name__)

if __name__ == "__main__":
    app = CloudflareSpeedTest()
    app.mainloop()