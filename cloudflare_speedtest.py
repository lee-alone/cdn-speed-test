import tkinter as tk
from tkinter import ttk, messagebox
import requests
import threading
import os
import random
import ipaddress
import time
import logging
from typing import List, Dict, Optional
from dataclasses import dataclass
import ssl

# 配置日志
logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s")
logger = logging.getLogger(__name__)

BASE_DIR = os.path.dirname(os.path.abspath(__file__))

@dataclass
class SpeedTestResult:
    ip: str
    status: str = "待测试"
    latency: str = "-"
    speed: str = "-"
    datacenter: str = "-"
    peak_speed: float = 0

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
    def check_files(cls) -> List[str]:
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

class IPGenerator:
    def __init__(self):
        self.used_ips = set()

    def generate_ipv4(self, subnet: str) -> Optional[str]:
        try:
            network = ipaddress.IPv4Network(subnet)
            if network.num_addresses <= 2:
                return None
            ip = str(random.choice(list(network.hosts())))
            if ip not in self.used_ips:
                self.used_ips.add(ip)
                return ip
            return None
        except Exception as e:
            logger.error(f"生成 IPv4 失败: {str(e)}")
            return None

    def generate_ipv6(self, subnet: str) -> Optional[str]:
        try:
            network = ipaddress.IPv6Network(subnet)
            hex_chars = '0123456789abcdef'
            suffix = ''.join(random.choice(hex_chars) for _ in range(16))
            ip = f"{network[0].exploded.rsplit(':', 1)[0]}:{suffix}"
            if ip not in self.used_ips:
                self.used_ips.add(ip)
                return ip
            return None
        except Exception as e:
            logger.error(f"生成 IPv6 失败: {str(e)}")
            return None

class SpeedTester:
    def __init__(self, parent):
        self.parent = parent
        self.domain = "example.com"  # 默认值
        self.file_path = ""
        self.load_url_config()

    def load_url_config(self):
        try:
            with open(FileDownloader.get_file_path('url.txt'), 'r', encoding='utf-8') as f:
                url = f.read().strip()
                parts = url.split('/', 1)
                self.domain = parts[0]
                self.file_path = parts[1] if len(parts) > 1 else ""
        except Exception as e:
            logger.warning(f"加载 URL 配置失败，使用默认值: {str(e)}")

    def test_speed(self, ip: str, use_tls: bool, timeout: int = 10) -> Dict:
        result = {"speed": 0, "latency": 0, "datacenter": "", "peak_speed": 0}
        protocol = "https" if use_tls else "http"
        port = "443" if use_tls else "80"
        
        try:
            session = requests.Session()
            if use_tls:
                session.verify = False
                
            # 测试延迟
            start_time = time.time()
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

            # 测速准备
            peak_speed = 0
            total_size = 0
            speed_samples = []
            download_duration = float(self.parent.download_time.get())
            chunk_size = 8192
            
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
                        
                        if current_speed > peak_speed:
                            peak_speed = current_speed
                            
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

class CloudflareSpeedTest(tk.Tk):
    def __init__(self):
        super().__init__()
        self.title("Cloudflare IP 优选测速")
        self.geometry("800x600")
        self.results_lock = threading.Lock()
        self.init_components()
        self.ip_generator = IPGenerator()
        self.speed_tester = SpeedTester(self)
        self.results: List[SpeedTestResult] = []
        self.testing = False
        self.test_thread = None
        self.protocol("WM_DELETE_WINDOW", self.on_closing)

    def init_components(self):
        control_frame = ttk.Frame(self)
        control_frame.pack(fill=tk.X, padx=5, pady=5)

        ttk.Label(control_frame, text="期待服务数:").pack(side=tk.LEFT, padx=5)
        self.expected_servers = ttk.Entry(control_frame, width=5)
        self.expected_servers.insert(0, "1")
        self.expected_servers.pack(side=tk.LEFT, padx=5)

        self.use_tls = tk.BooleanVar(value=False)
        ttk.Checkbutton(control_frame, text="启用TLS", variable=self.use_tls).pack(side=tk.LEFT, padx=5)

        self.ip_type = tk.StringVar(value="ipv4")
        ttk.Radiobutton(control_frame, text="IPv4", variable=self.ip_type, value="ipv4").pack(side=tk.LEFT, padx=5)
        ttk.Radiobutton(control_frame, text="IPv6", variable=self.ip_type, value="ipv6").pack(side=tk.LEFT, padx=5)

        ttk.Label(control_frame, text="期望带宽(Mbps):").pack(side=tk.LEFT, padx=5)
        self.bandwidth = ttk.Entry(control_frame, width=10)
        self.bandwidth.insert(0, "5")
        self.bandwidth.pack(side=tk.LEFT, padx=5)

        ttk.Label(control_frame, text="超时(s):").pack(side=tk.LEFT, padx=5)
        self.timeout = ttk.Entry(control_frame, width=5)
        self.timeout.insert(0, "5")
        self.timeout.pack(side=tk.LEFT, padx=5)

        ttk.Label(control_frame, text="下载时间(s):").pack(side=tk.LEFT, padx=5)
        self.download_time = ttk.Entry(control_frame, width=5)
        self.download_time.insert(0, "15")
        self.download_time.pack(side=tk.LEFT, padx=5)

        self.tree = ttk.Treeview(self, columns=("ip", "status", "latency", "speed", "datacenter", "peak_speed"), show="headings")
        self.tree.heading("ip", text="IP地址")
        self.tree.heading("status", text="状态")
        self.tree.heading("latency", text="延迟(ms)")
        self.tree.heading("speed", text="速度(Mbps)")
        self.tree.heading("datacenter", text="数据中心")
        self.tree.heading("peak_speed", text="峰值速度(Mbps)")
        self.tree.pack(fill=tk.BOTH, expand=True, padx=5, pady=5)

        scrollbar = ttk.Scrollbar(self, orient=tk.VERTICAL, command=self.tree.yview)
        scrollbar.pack(side=tk.RIGHT, fill=tk.Y)
        self.tree.configure(yscrollcommand=scrollbar.set)

        button_frame = ttk.Frame(self)
        button_frame.pack(fill=tk.X, padx=5, pady=5)

        self.progress_label = ttk.Label(button_frame, text="测试进度: 0/0")
        self.progress_label.pack(side=tk.LEFT, padx=5)

        self.current_speed_label = ttk.Label(button_frame, text="当前速度: - Mbps")
        self.current_speed_label.pack(side=tk.LEFT, padx=5)

        ttk.Button(button_frame, text="更新数据", command=self.update_files).pack(side=tk.LEFT, padx=5)
        self.start_button = ttk.Button(button_frame, text="开始测试", command=self.start_test)
        self.start_button.pack(side=tk.LEFT, padx=5)
        self.stop_button = ttk.Button(button_frame, text="停止测试", command=self.stop_test, state=tk.DISABLED)
        self.stop_button.pack(side=tk.LEFT, padx=5)

        self.clear_button = ttk.Button(button_frame, text="清空任务", command=self.clear_task)
        self.clear_button.pack(side=tk.LEFT, padx=5)

        self.save_button = ttk.Button(button_frame, text="保存结果", command=self.save_results)
        self.save_button.pack(side=tk.LEFT, padx=5)

    def clear_task(self):
        self.testing = False
        self.start_button.config(state=tk.NORMAL)
        self.stop_button.config(state=tk.DISABLED)
        with self.results_lock:
            for item in self.tree.get_children():
                self.tree.delete(item)
            self.results = []
        self.progress_label.config(text="测试进度: 0/0")
        self.current_speed_label.config(text="当前速度: - Mbps")

    def update_files(self):
        missing_files = FileDownloader.check_files()
        if not missing_files:
            messagebox.showinfo("提示", "所有文件已存在且是最新的")
            return

        def download():
            for filename in missing_files:
                self.progress_label.config(text=f"正在下载: {filename}")
                success = FileDownloader.download_file(filename, FileDownloader.URLS[filename])
                if not success:
                    messagebox.showerror("错误", f"下载 {filename} 失败，请手动下载")
                    return
            self.progress_label.config(text="文件更新完成")
            messagebox.showinfo("提示", "文件更新完成")

        threading.Thread(target=download, daemon=True).start()

    def start_test(self):
        if not all(os.path.exists(FileDownloader.get_file_path(f)) for f in FileDownloader.URLS.keys()):
            messagebox.showerror("错误", "缺少必要文件，请先更新数据")
            return

        try:
            self.expected_bandwidth = float(self.bandwidth.get())
            if self.expected_bandwidth <= 0:
                raise ValueError("带宽必须为正数")
            self.test_timeout = int(self.timeout.get())
            if self.test_timeout <= 0:
                raise ValueError("超时必须为正数")
            self.expected_servers_count = int(self.expected_servers.get())
            if self.expected_servers_count <= 0:
                raise ValueError("期待服务数必须为正数")
        except ValueError as e:
            messagebox.showerror("错误", str(e))
            return

        self.testing = True
        self.start_button.config(state=tk.DISABLED)
        self.stop_button.config(state=tk.NORMAL)
        self.test_thread = threading.Thread(target=self.test_process, daemon=True)
        self.test_thread.start()

    def stop_test(self):
        self.testing = False
        self.start_button.config(state=tk.NORMAL)
        self.stop_button.config(state=tk.DISABLED)

    def update_tree(self, item, result: SpeedTestResult):
        with self.results_lock:
            self.tree.set(item, column="status", value=result.status)
            self.tree.set(item, column="latency", value=result.latency)
            self.tree.set(item, column="speed", value=result.speed)
            self.tree.set(item, column="datacenter", value=result.datacenter)
            self.tree.set(item, column="peak_speed", value=f"{float(result.peak_speed):.2f}")

    def update_progress(self, current: int, total: int):
        self.progress_label.config(text=f"测试进度: {current}/{total}")

    def test_process(self):
        ip_file = FileDownloader.get_file_path("ips-v4.txt" if self.ip_type.get() == "ipv4" else "ips-v6.txt")
        generate_func = self.ip_generator.generate_ipv4 if self.ip_type.get() == "ipv4" else self.ip_generator.generate_ipv6

        while self.testing:
            with open(ip_file, 'r', encoding='utf-8') as f:
                subnets = f.read().splitlines()

            # 每次只生成一个IP进行测试
            ip = None
            for subnet in random.sample(subnets, min(len(subnets), 10)):
                if not self.testing:
                    break
                ip = generate_func(subnet)
                if ip:
                    break

            if not ip:
                continue

            # 添加新的测试项到列表
            with self.results_lock:
                result = SpeedTestResult(ip=ip)
                self.results.append(result)
                item = self.tree.insert("", tk.END, values=(ip, "待测试", "-", "-", "-", "-"))

            if not self.testing:
                break

            # 开始测试当前IP
            self.tree.set(item, column="status", value="测试中")
            test_result = self.speed_tester.test_speed(ip, self.use_tls.get(), self.test_timeout)
            
            # 更新测试结果
            result.status = "已完成"
            if isinstance(test_result['latency'], (int, float)):
                result.latency = f"{test_result['latency']:.2f}"
            else:
                result.latency = test_result['latency']
            result.speed = f"{test_result['speed']:.2f}"
            result.datacenter = test_result['datacenter']
            result.peak_speed = test_result['peak_speed']
            
            self.update_tree(item, result)
            self.update_progress(len(self.results), len(self.results))

            # 检查是否已找到满足要求的服务器
            qualified_servers = [r for r in self.results if r.status == "已完成" and float(r.speed) >= self.expected_bandwidth]
            if len(qualified_servers) >= self.expected_servers_count:
                found_ips = [f"{r.ip} - {r.speed}Mbps" for r in qualified_servers]
                messagebox.showinfo("测试完成", "找到满足要求的IP:\n" + "\n".join(found_ips))
                self.after(0, self.stop_test)
                self.save_results()
                return

    def save_result(self, result: SpeedTestResult):
        now = time.strftime("%Y-%m-%d %H:%M:%S", time.localtime())
        with open(FileDownloader.get_file_path('find.txt'), 'a', encoding='utf-8') as f:
            f.write(f"[{now}] 优选IP: {result.ip}\n延迟: {result.latency}\n速度: {result.speed}Mbps\n数据中心: {result.datacenter}\n")
            ports = "443,2053,2083,2087,2096,8443" if self.use_tls.get() else "80,8080,8880,2052,2082,2086,2095"
            f.write(f"[{now}] 支持端口: {ports}\n")

    def save_results(self):
        # 筛选符合要求的服务器
        qualified_servers = [r for r in self.results if r.status == "已完成" and float(r.speed) >= self.expected_bandwidth]
        
        if not qualified_servers:
            messagebox.showinfo("提示", "没有找到符合要求的服务器")
            return
            
        now = time.strftime("%Y-%m-%d %H:%M:%S", time.localtime())
        filepath = FileDownloader.get_file_path('find.txt')
        try:
            with open(filepath, 'a', encoding='utf-8') as f:
                f.write(f"[{now}] 符合要求的服务器:\n")
                f.write(f"期望带宽: {self.expected_bandwidth}Mbps\n")
                for result in qualified_servers:
                    f.write(f"IP: {result.ip}\n")
                    f.write(f"延迟: {result.latency}ms\n")
                    f.write(f"平均速度: {result.speed}Mbps\n")
                    f.write(f"峰值速度: {float(result.peak_speed):.2f}Mbps\n")
                    f.write(f"数据中心: {result.datacenter}\n")
                    ports = "443,2053,2083,2087,2096,8443" if self.use_tls.get() else "80,8080,8880,2052,2082,2086,2095"
                    f.write(f"支持端口: {ports}\n")
                    f.write("-" * 20 + "\n")
            messagebox.showinfo("提示", f"已将{len(qualified_servers)}个符合要求的服务器信息保存到 {filepath}")
        except Exception as e:
            messagebox.showerror("错误", f"保存结果失败: {str(e)}")

    def on_closing(self):
        self.stop_test()
        if self.test_thread and self.test_thread.is_alive():
            self.test_thread.join(timeout=1)
        self.destroy()

if __name__ == "__main__":
    app = CloudflareSpeedTest()
    app.mainloop()