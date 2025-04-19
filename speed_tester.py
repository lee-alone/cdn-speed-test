import requests
import time
import logging
import os
from downloader import FileDownloader

logger = logging.getLogger(__name__)

class SpeedTester:
    def __init__(self, parent):
        self.parent = parent
        self.domain = "example.com"  # 默认值
        self.file_path = ""
        self.load_url_config()

    def load_url_config(self):
        try:
            file_path = FileDownloader.get_file_path('url.txt')
            if not os.path.exists(file_path):
                logger.warning(f"文件不存在: {file_path}")
                self.domain = "example.com"
                self.file_path = ""
                return
            with open(file_path, 'r', encoding='utf-8') as f:
                url = f.read().strip()
                parts = url.split('/', 1)
                self.domain = parts[0]
                self.file_path = parts[1] if len(parts) > 1 else ""
        except Exception as e:
            logger.warning(f"加载 URL 配置失败，使用默认值: {str(e)}")

    def test_speed(self, ip: str, use_tls: bool, timeout: int = 10, datacenter_only: bool = False) -> dict:
        result = {"speed": 0, "latency": 0, "datacenter": "", "peak_speed": 0}
        protocol = "https" if use_tls else "http"
        port = "443" if use_tls else "80"
        
        try:
            session = requests.Session()
            if use_tls:
                session.verify = False
                
            # 测试延迟
            start_time = time.time()
            import ipaddress
            
            try:
                ipaddress.ip_address(ip)
                if isinstance(ipaddress.ip_address(ip), ipaddress.IPv6Address):
                    ip = f"[{ip}]"
            except ValueError:
                pass

            response = session.get(
                f"{protocol}://{ip}/cdn-cgi/trace",
                headers={'Host': self.domain},
                timeout=timeout // 2,
                allow_redirects=True
            )
            result["latency"] = round((time.time() - start_time) * 1000, 2)
            
            if response.status_code == 200:
                for line in response.text.split('\n'):
                    if line.startswith('colo='):
                        result["datacenter"] = line.split('=')[1]
            
            # 如果只需要检测数据中心，则直接返回结果
            if datacenter_only:
                return result

            # 测速准备
            peak_speed = 0
            total_size = 0
            speed_samples = []
            window_size = 5  # 滑动窗口大小
            download_duration = float(self.parent.download_time.get())
            chunk_size = 65536
            
            response = session.get(
                f"{protocol}://{ip}/{self.file_path}",
                headers={'Host': self.domain},
                timeout=timeout,
                stream=True
            )

            start_time = time.time()
            end_time = start_time + download_duration
            last_update_time = start_time

            try:
                for chunk in response.iter_content(chunk_size=chunk_size):
                    current_time = time.time()
                    if current_time > end_time:
                        break
                        
                    chunk_size = len(chunk)
                    total_size += chunk_size
                    
                    # 每0.5秒更新一次显示
                    if current_time - last_update_time >= 0.5:
                        elapsed = current_time - start_time
                        current_speed = (total_size / 1024 / elapsed) / 128  # Mbps
                        speed_samples.append(current_speed)
                        if len(speed_samples) > window_size:
                            speed_samples.pop(0)  # 移除最早的样本
                        
                        # 计算滑动窗口内的平均速度
                        average_speed = sum(speed_samples) / len(speed_samples) if speed_samples else 0
                        
                        if average_speed > peak_speed:
                            peak_speed = average_speed
                            
                        # 更新UI显示
                        self.parent.after(0, lambda s=current_speed, p=peak_speed: 
                            self.parent.current_speed_label.config(
                                text=f"当前速度: {s:.2f} Mbps, 峰值速度: {p:.2f} Mbps"
                            )
                        )
                        last_update_time = current_time

                # 计算最终结果
                total_duration = time.time() - start_time
                if total_duration > 0 and total_size > 0:
                    # 计算平均速度
                    result["speed"] = round((total_size / 1024 / total_duration) / 128, 2)  # Mbps
                    result["peak_speed"] = round(peak_speed, 2)
                
            except requests.exceptions.RequestException as e:
                logger.error(f"测速过程中出错: {str(e)}")
                if total_size > 0 and (time.time() - start_time) > 0:
                    result["speed"] = round((total_size / 1024 / (time.time() - start_time)) / 128, 2)
                    result["peak_speed"] = round(peak_speed, 2)
                
        except Exception as e:
            logger.error(f"测速 {ip} 失败: {str(e)}")
            result["latency"] = "timeout"
            
        return result