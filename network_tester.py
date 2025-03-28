import requests
import logging

logger = logging.getLogger(__name__)

def check_network_environment():
    """
    检查网络环境，通过访问 http://www.gstatic.com/generate_204 判断是否需要科学上网
    Returns:
        bool: True 表示需要关闭科学上网，False 表示当前环境正常
    """
    try:
        response = requests.get('http://www.gstatic.com/generate_204', timeout=2)
        return response.status_code == 204  # 如果能访问并返回204，说明需要关闭科学上网
    except requests.RequestException:
        return False  # 如果无法访问，说明当前环境正常