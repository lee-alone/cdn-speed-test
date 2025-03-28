import requests
import os
import logging

logger = logging.getLogger(__name__)

BASE_DIR = os.path.dirname(os.path.abspath(__file__))

class FileDownloader:
    URLS = {
        "colo.txt": "https://www.baipiao.eu.org/cloudflare/colo",
        "url.txt": "https://www.baipiao.eu.org/cloudflare/url",
        "ips-v4.txt": "https://www.baipiao.eu.org/cloudflare/ips-v4",
        "ips-v6.txt": "https://www.baipiao.eu.org/cloudflare/ips-v6"
    }

    @staticmethod
    def get_file_path(filename: str) -> str:
        return os.path.join(BASE_DIR, filename)

    @classmethod
    def check_files(cls) -> list[str]:
        return list(cls.URLS.keys())

    @classmethod
    def download_file(cls, filename: str, url: str) -> bool:
        try:
            response = requests.get(url, timeout=10)
            response.raise_for_status()
            with open(cls.get_file_path(filename), 'w', encoding='utf-8') as f:
                f.write(response.text)
            logger.info(f"成功下载 {filename}")
            return True
        except Exception as e:
            logger.error(f"下载 {filename} 失败: {str(e)}")
            return False