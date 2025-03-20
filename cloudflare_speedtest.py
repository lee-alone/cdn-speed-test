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
        return [f for f in cls.URLS.keys() if not os.path.exists(cls.get_file_path(f))]

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
        result = {"speed": 0, "latency": 0, "datacenter": ""}
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

            # 测试速度
            start_time = time.time()
            total_size = 0
            download_duration = float(self.parent.download_time.get())  # 获取下载持续时间
            
            response = session.get(
                f"{protocol}://{ip}/{self.file_path}",
                headers={'Host': self.domain},
                timeout=timeout,
                stream=True
            )
            
            start_time = time.time()
            end_time = start_time + download_duration
            try:
                for chunk in response.iter_content(chunk_size=8192):
                    total_size += len(chunk)
                    current_time = time.time() - start_time
                    if current_time > 0:
                        current_speed = round((total_size / 1024 / current_time) / 128, 2)  # Mbps
                        self.parent.after(0, lambda speed=current_speed: self.parent.current_speed_label.config(text=f"当前速度: {speed:.2f} Mbps"))
                    if time.time() > end_time:
                        break
                duration = time.time() - start_time
            except requests.exceptions.RequestException as e:
                logger.error(f"测速 {ip} 失败: {str(e)}")
                result["latency"] = "time out"
            else:
                if duration > 0:
                    result["speed"] = round((total_size / 1024 / duration) / 128, 2)  # Mbps
        except Exception as e:
            logger.error(f"测速 {ip} 失败: {str(e)}")
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

        self.tree = ttk.Treeview(self, columns=("ip", "status", "latency", "speed", "datacenter"), show="headings")
        self.tree.heading("ip", text="IP地址")
        self.tree.heading("status", text="状态")
        self.tree.heading("latency", text="延迟(ms)")
        self.tree.heading("speed", text="速度(Mbps)")
        self.tree.heading("datacenter", text="数据中心")
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
                if not FileDownloader.download_file(filename, FileDownloader.URLS[filename]):
                    messagebox.showerror("错误", f"下载 {filename} 失败，请手动下载")
                    return
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

    def update_progress(self, current: int, total: int):
        self.progress_label.config(text=f"测试进度: {current}/{total}")

    def test_process(self):
        ip_file = FileDownloader.get_file_path("ips-v4.txt" if self.ip_type.get() == "ipv4" else "ips-v6.txt")
        generate_func = self.ip_generator.generate_ipv4 if self.ip_type.get() == "ipv4" else self.ip_generator.generate_ipv6

        while self.testing:
            with open(ip_file, 'r', encoding='utf-8') as f:
                subnets = f.read().splitlines()

            ips = []
            for subnet in random.sample(subnets, min(len(subnets), 10)):
                ip = generate_func(subnet)
                if ip and ip not in ips:
                    ips.append(ip)
                if len(ips) >= 10:
                    break

            with self.results_lock:
                for ip in ips:
                    result = SpeedTestResult(ip=ip)
                    self.results.append(result)
                    self.tree.insert("", tk.END, values=(ip, "待测试", "-", "-", "-"))

            for i, result in enumerate(self.results):
                if not self.testing:
                    break
                if result.status != "待测试":
                    continue

                item = self.tree.get_children()[i]
                self.after(0, lambda it=item: self.tree.set(it, column="status", value="测试中"))
                test_result = self.speed_tester.test_speed(result.ip, self.use_tls.get(), self.test_timeout)

                result.status = "已完成"
                result.latency = f"{test_result['latency']:.2f}"
                result.speed = f"{test_result['speed']:.2f}"
                result.datacenter = test_result['datacenter']
                self.after(0, lambda it=item, res=result: self.update_tree(it, res))
                self.after(0, lambda: self.update_progress(i + 1, len(self.results)))

                if test_result['speed'] >= self.expected_bandwidth:
                    self.save_result(result)
                    
                    if len([r for r in self.results if r.status == "已完成" and float(r.speed) >= self.expected_bandwidth]) >= self.expected_servers_count:
                        
                        found_ips = [f"{r.ip} - {r.speed}Mbps" for r in self.results if r.status == "已完成" and float(r.speed) >= self.expected_bandwidth]
                        messagebox.showinfo("测试完成", "找到满足要求的IP:\n" + "\n".join(found_ips))
                        self.after(0, self.stop_test)
                        break

    def save_result(self, result: SpeedTestResult):
        now = time.strftime("%Y-%m-%d %H:%M:%S", time.localtime())
        with open(FileDownloader.get_file_path('find.txt'), 'a', encoding='utf-8') as f:
            f.write(f"[{now}] 优选IP: {result.ip}\n延迟: {result.latency}\n速度: {result.speed}Mbps\n数据中心: {result.datacenter}\n")
            ports = "443,2053,2083,2087,2096,8443" if self.use_tls.get() else "80,8080,8880,2052,2082,2086,2095"
            f.write(f"[{now}] 支持端口: {ports}\n")
    def on_closing(self):
        self.stop_test()
        if self.test_thread and self.test_thread.is_alive():
            self.test_thread.join(timeout=1)
        self.destroy()

if __name__ == "__main__":
    app = CloudflareSpeedTest()
    app.mainloop()