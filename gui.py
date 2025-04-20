import tkinter as tk
from tkinter import ttk, messagebox, filedialog
import threading
import os
import random
import time
import logging
from typing import List, Dict, Optional
from dataclasses import dataclass

from ip_generator import IPGenerator
from speed_tester import SpeedTester
from downloader import FileDownloader
from config_manager import load_config, save_config, load_find_path
from result_writer import save_results
from network_tester import check_network_environment

logger = logging.getLogger(__name__)

@dataclass
class SpeedTestResult:
    ip: str
    status: str = "待测试"
    latency: str = "-"
    speed: str = "-"
    datacenter: str = "-"
    peak_speed: float = 0

class CloudflareSpeedTest(tk.Tk):
    TLS_PORTS = "443,2053,2083,2087,2096,8443"
    NON_TLS_PORTS = "80,8080,8880,2052,2082,2086,2095"

    def __init__(self):
        super().__init__()
        self.title("Cloudflare IP 优选测速")
        self.geometry("1024x768")  # 增加窗口大小
        
        # 添加排序相关变量
        self.sort_column = None
        self.sort_reverse = False

        # 设置窗口样式和主题
        self.style = ttk.Style()
        self.style.theme_use('clam')
        
        # 自定义颜色主题
        self.style.configure("TFrame", background="#f0f0f0")
        self.style.configure("TLabel", background="#f0f0f0", font=('微软雅黑', 9))
        self.style.configure("TButton", padding=6, relief="flat", background="#4a90e2", foreground="white")
        self.style.map("TButton",
            background=[('active', '#357abd'), ('disabled', '#cccccc')],
            foreground=[('disabled', '#666666')])
        self.style.configure("Treeview", background="white", 
                        fieldbackground="white", foreground="black")
        self.style.configure("Treeview.Heading", font=('微软雅黑', 9, 'bold'))
        self.style.configure("Network.TLabel", foreground="red", font=('微软雅黑', 9, 'bold'))
        
        self.configure(bg="#f0f0f0")
        random.seed(time.time())
        self.results_lock = threading.Lock()
        
        # 读取配置文件
        self.config = load_config()
        self.filepath = self.config['DEFAULT'].get('filepath', '')
        
        # 加载数据中心信息
        self.datacenters = self.load_datacenters()
        self.all_datacenters = self.datacenters.copy()  # 保存完整列表供筛选使用

        self.init_components()
        self.ip_generator = IPGenerator()
        self.speed_tester = SpeedTester(self)
        self.results: List[SpeedTestResult] = []
        self.testing = False
        self.test_thread = None
        
        # 添加网络环境检测
        self.check_network_environment()
        self.protocol("WM_DELETE_WINDOW", self.on_closing)

    def load_datacenters(self):
        """从colo.txt文件中加载数据中心信息"""
        datacenters = []  # 临时列表，不包含"全部"选项
        try:
            file_path = FileDownloader.get_file_path('colo.txt')
            if os.path.exists(file_path):
                with open(file_path, 'r', encoding='utf-8') as f:
                    for line in f:
                        line = line.strip()
                        if line:
                            parts = line.split(',')
                            if len(parts) == 2:
                                location, code = parts
                                # 提取缩写部分，去掉区域信息
                                code_only = code.split('-')[-1].strip('()')
                                # 数据中心缩写放在前面，其他信息放在后面
                                datacenters.append(f"{code_only} - {location}")
                
                # 按照数据中心代码（前缀部分）排序
                datacenters.sort(key=lambda x: x.split(' - ')[0])
        except Exception as e:
            logger.error(f"加载数据中心信息失败: {str(e)}")
        
        # 在排序后的列表前添加"全部"选项
        return ["全部"] + datacenters

    def init_components(self):
        # 创建主容器
        main_container = ttk.Frame(self)
        main_container.pack(fill=tk.BOTH, expand=True, padx=10, pady=10)
        
        # 创建上部配置区域
        config_frame = ttk.LabelFrame(main_container, text="配置选项", padding=(10, 5))
        config_frame.pack(fill=tk.X, padx=5, pady=(0, 10))
        
        # 第一行配置
        row1_frame = ttk.Frame(config_frame)
        row1_frame.pack(fill=tk.X, pady=5)
        
        # 保存路径
        path_frame = ttk.Frame(row1_frame)
        path_frame.pack(side=tk.LEFT, fill=tk.X, expand=True)
        ttk.Label(path_frame, text="保存路径:").pack(side=tk.LEFT, padx=5)
        self.find_path_label = ttk.Label(path_frame, text=self.config['DEFAULT']['filepath'])
        self.find_path_label.pack(side=tk.LEFT, padx=5)
        
        # IP类型选择
        ip_frame = ttk.Frame(row1_frame)
        ip_frame.pack(side=tk.RIGHT)
        self.ip_type = tk.StringVar(value=self.config['DEFAULT']['ip_type'])
        ttk.Radiobutton(ip_frame, text="IPv4", variable=self.ip_type, value="ipv4").pack(side=tk.LEFT, padx=5)
        ttk.Radiobutton(ip_frame, text="IPv6", variable=self.ip_type, value="ipv6").pack(side=tk.LEFT, padx=5)
        
        # 第二行配置
        row2_frame = ttk.Frame(config_frame)
        row2_frame.pack(fill=tk.X, pady=5)
        
        # 左侧配置组
        left_frame = ttk.Frame(row2_frame)
        left_frame.pack(side=tk.LEFT)
        
        ttk.Label(left_frame, text="期待服务数:").pack(side=tk.LEFT, padx=5)
        self.expected_servers = ttk.Entry(left_frame, width=5)
        self.expected_servers.insert(0, self.config['DEFAULT']['expected_servers'])
        self.expected_servers.pack(side=tk.LEFT, padx=5)
        
        self.use_tls = tk.BooleanVar(value=self.config.getboolean('DEFAULT', 'use_tls'))
        ttk.Checkbutton(left_frame, text="启用TLS", variable=self.use_tls).pack(side=tk.LEFT, padx=10)
        
        # 数据中心选择
        ttk.Label(left_frame, text="数据中心:").pack(side=tk.LEFT, padx=5)
        self.datacenter_var = tk.StringVar(value="全部")
        self.datacenter_combobox = ttk.Combobox(left_frame, textvariable=self.datacenter_var, width=20)
        self.datacenter_combobox['values'] = self.datacenters
        self.datacenter_combobox.current(0)  # 默认选择"全部"
        self.datacenter_combobox.pack(side=tk.LEFT, padx=5)
        
        # 绑定键盘事件
        self.datacenter_combobox.bind('<KeyRelease>', self.filter_datacenters)
        self.all_datacenters = self.datacenters.copy()  # 保存完整的数据中心列表
        
        # 网络环境提示标签
        self.network_label = ttk.Label(left_frame, text="", style="Network.TLabel")
        self.network_label.pack(side=tk.LEFT, padx=10)
        
        # 右侧配置组
        right_frame = ttk.Frame(row2_frame)
        right_frame.pack(side=tk.RIGHT)
        
        ttk.Label(right_frame, text="期望带宽(Mbps):").pack(side=tk.LEFT, padx=5)
        self.bandwidth = ttk.Entry(right_frame, width=8)
        self.bandwidth.insert(0, self.config['DEFAULT']['bandwidth'])
        self.bandwidth.pack(side=tk.LEFT, padx=5)
        
        ttk.Label(right_frame, text="超时(s):").pack(side=tk.LEFT, padx=5)
        self.timeout = ttk.Entry(right_frame, width=5)
        self.timeout.insert(0, self.config['DEFAULT']['timeout'])
        self.timeout.pack(side=tk.LEFT, padx=5)
        
        ttk.Label(right_frame, text="下载时间(s):").pack(side=tk.LEFT, padx=5)
        self.download_time = ttk.Entry(right_frame, width=5)
        self.download_time.insert(0, self.config['DEFAULT']['download_time'])
        self.download_time.pack(side=tk.LEFT, padx=5)
        
        # 创建表格区域
        table_frame = ttk.Frame(main_container)
        table_frame.pack(fill=tk.BOTH, expand=True)
        
        # 配置表格列宽
        self.tree = ttk.Treeview(table_frame, columns=("ip", "status", "latency", "speed", "datacenter", "peak_speed"), show="headings")
        self.tree.heading("ip", text="IP地址", command=lambda: self.sort_tree("ip"))
        self.tree.heading("status", text="状态", command=lambda: self.sort_tree("status"))
        self.tree.heading("latency", text="延迟(ms)", command=lambda: self.sort_tree("latency"))
        self.tree.heading("speed", text="速度(Mbps)", command=lambda: self.sort_tree("speed"))
        self.tree.heading("datacenter", text="数据中心", command=lambda: self.sort_tree("datacenter"))
        self.tree.heading("peak_speed", text="峰值速度(Mbps)", command=lambda: self.sort_tree("peak_speed"))
        
        # 设置列宽
        self.tree.column("ip", width=150)
        self.tree.column("status", width=80)
        self.tree.column("latency", width=100)
        self.tree.column("speed", width=100)
        self.tree.column("datacenter", width=150)
        self.tree.column("peak_speed", width=120)
        
        self.tree.pack(side=tk.LEFT, fill=tk.BOTH, expand=True)
        
        # 添加滚动条
        scrollbar = ttk.Scrollbar(table_frame, orient=tk.VERTICAL, command=self.tree.yview)
        scrollbar.pack(side=tk.RIGHT, fill=tk.Y)
        self.tree.configure(yscrollcommand=scrollbar.set)
        
        # 创建底部状态和按钮区域
        bottom_frame = ttk.Frame(main_container)
        bottom_frame.pack(fill=tk.X, pady=(10, 0))
        
        # 状态信息
        status_frame = ttk.Frame(bottom_frame)
        status_frame.pack(side=tk.LEFT)
        
        self.progress_label = ttk.Label(status_frame, text="测试进度: 0/0")
        self.progress_label.pack(side=tk.LEFT, padx=5)
        
        self.current_speed_label = ttk.Label(status_frame, text="当前速度: - Mbps")
        self.current_speed_label.pack(side=tk.LEFT, padx=15)
        
        # 按钮区域
        button_frame = ttk.Frame(bottom_frame)
        button_frame.pack(side=tk.RIGHT)
        
        self.update_button = ttk.Button(button_frame, text="更新数据", command=self.update_files)
        self.update_button.pack(side=tk.LEFT, padx=5)
        
        self.start_button = ttk.Button(button_frame, text="开始测试", command=self.start_test)
        self.start_button.pack(side=tk.LEFT, padx=5)
        
        self.stop_button = ttk.Button(button_frame, text="停止测试", command=self.stop_test, state=tk.DISABLED)
        self.stop_button.pack(side=tk.LEFT, padx=5)
        
        self.clear_button = ttk.Button(button_frame, text="清空任务", command=self.clear_task)
        self.clear_button.pack(side=tk.LEFT, padx=5)
        
        self.save_button = ttk.Button(button_frame, text="结果路径", command=self.choose_path)
        self.save_button.pack(side=tk.LEFT, padx=5)

    def choose_path(self):
        self.filepath = filedialog.askdirectory()
        if self.filepath:
            config = load_config()
            save_config(config, self.expected_servers.get(), self.use_tls.get(), self.ip_type.get(), self.bandwidth.get(), self.timeout.get(), self.download_time.get(), self.filepath)
            self.find_path_label.config(text=self.filepath)
        else:
            config = load_config()
            self.filepath = config['DEFAULT'].get('filepath', '')

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
        missing_files = [f for f in FileDownloader.URLS.keys() if not os.path.exists(FileDownloader.get_file_path(f))]
        if missing_files:
            messagebox.showerror("错误", f"缺少文件: {', '.join(missing_files)}，请先更新数据")
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

        config = load_config()
        save_config(config, self.expected_servers.get(), self.use_tls.get(), self.ip_type.get(), self.bandwidth.get(), self.timeout.get(), self.download_time.get(), self.filepath)
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

    def test_process(self):
        ip_file = FileDownloader.get_file_path("ips-v4.txt" if self.ip_type.get() == "ipv4" else "ips-v6.txt")
        ip_type = self.ip_type.get()
        generate_func = lambda subnet: self.ip_generator.generate_ip(subnet, ip_type)
        selected_datacenter = self.datacenter_var.get()
        use_tls = self.use_tls.get()
        
        # 提取用户选择的数据中心代码
        selected_datacenter_code = ""
        if selected_datacenter != "全部":
            # 从选项中提取数据中心代码
            selected_datacenter_code = selected_datacenter.split(" - ")[0].strip()

        while self.testing:
            # 首先检查是否已经找到足够的合格IP
            qualified_servers = [r for r in self.results if r.status == "已完成" and r.speed != "-" and float(r.speed) >= self.expected_bandwidth]
            if len(qualified_servers) >= self.expected_servers_count:
                save_results(qualified_servers, self.expected_bandwidth, self.filepath, use_tls, self.TLS_PORTS, self.NON_TLS_PORTS)
                found_ips = [f"{r.ip} - {r.speed}Mbps" for r in qualified_servers]
                messagebox.showinfo("测试完成", "找到满足要求的IP:\n" + "\n".join(found_ips))
                self.after(0, self.stop_test)
                return

            with open(ip_file, 'r', encoding='utf-8') as f:
                subnets = f.read().splitlines()
            
            # 随机抽取IP进行预测试
            candidate_ips = []
            tested_subnets = set()
            
            # 随机选择子网并生成IP
            for _ in range(min(30, len(subnets))):
                if not self.testing:
                    return
                    
                subnet = random.choice(subnets)
                if subnet in tested_subnets:
                    continue
                    
                tested_subnets.add(subnet)
                ip = generate_func(subnet)
                if ip:
                    candidate_ips.append(ip)
                    if len(candidate_ips) >= 10:
                        break
            
            if not candidate_ips or not self.testing:
                continue
                
            # 测试这些IP的数据中心
            matching_ips = []
            for ip in candidate_ips:
                if not self.testing:
                    return
                    
                # 添加新的测试项到列表
                with self.results_lock:
                    result = SpeedTestResult(ip=ip)
                    self.results.append(result)
                    item = self.tree.insert("", tk.END, values=(ip, "检测数据中心", "-", "-", "-", "-"))
                
                # 只测试数据中心信息
                self.tree.set(item, column="status", value="检测数据中心")
                datacenter_result = self.speed_tester.test_speed(ip, use_tls, self.test_timeout // 2, datacenter_only=True)
                datacenter_code = datacenter_result['datacenter']
                
                # 更新数据中心信息
                result.datacenter = datacenter_code
                self.tree.set(item, column="datacenter", value=datacenter_code)
                
                # 如果用户选择了特定数据中心，检查是否匹配
                if (selected_datacenter == "全部" or 
                    (datacenter_code and selected_datacenter_code and datacenter_code == selected_datacenter_code)):
                    matching_ips.append((ip, item, result))
                    result.status = "待测试"
                    self.tree.set(item, column="status", value="待测试")
                else:
                    result.status = "跳过"
                    self.tree.set(item, column="status", value="跳过")
            
            # 测试匹配的IP
            if matching_ips:
                for ip, item, result in matching_ips:
                    if not self.testing:
                        return
                        
                    # 在测试每个IP前检查是否已经找到足够的合格IP
                    qualified_servers = [r for r in self.results if r.status == "已完成" and r.speed != "-" and float(r.speed) >= self.expected_bandwidth]
                    if len(qualified_servers) >= self.expected_servers_count:
                        save_results(qualified_servers, self.expected_bandwidth, self.filepath, use_tls, self.TLS_PORTS, self.NON_TLS_PORTS)
                        found_ips = [f"{r.ip} - {r.speed}Mbps" for r in qualified_servers]
                        messagebox.showinfo("测试完成", "找到满足要求的IP:\n" + "\n".join(found_ips))
                        self.after(0, self.stop_test)
                        return
                        
                    # 开始测试当前IP
                    self.tree.set(item, column="status", value="测试中")
                    test_result = self.speed_tester.test_speed(ip, use_tls, self.test_timeout)
                    
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
                    self.progress_label.config(text=f"测试进度: {len([r for r in self.results if r.status == '已完成'])}/{len(self.results)}")

    def sort_tree(self, column):
        with self.results_lock:
            if self.sort_column == column:
                self.sort_reverse = not self.sort_reverse
            else:
                self.sort_column = column
                self.sort_reverse = False

            data = [(self.tree.set(child, column), child) for child in self.tree.get_children()]
            try:
                data.sort(key=lambda x: float(x[0]) if x[0].replace('.', '', 1).isdigit() else x[0], reverse=self.sort_reverse)
            except ValueError:
                data.sort(key=lambda x: x[0], reverse=self.sort_reverse)

            for index, (_, child) in enumerate(data):
                self.tree.move(child, '', index)

    def check_network_environment(self):
        """检查网络环境并更新UI"""
        def update_network_status():
            needs_proxy = check_network_environment()
            if needs_proxy:
                self.network_label.config(text="请关闭科学环境")
            else:
                self.network_label.config(text="当前环境正常")
            # 每60秒检查一次网络环境
            self.after(60000, update_network_status)
        
        update_network_status()

    def on_closing(self):
        config = load_config()
        save_config(config, self.expected_servers.get(), self.use_tls.get(), self.ip_type.get(), self.bandwidth.get(), self.timeout.get(), self.download_time.get(), self.filepath)
        self.stop_test()
        if self.test_thread and self.test_thread.is_alive():
            self.test_thread.join(timeout=1)
        self.destroy()

    def filter_datacenters(self, event):
        """根据用户输入筛选数据中心列表，只匹配缩写的首字母"""
        text = self.datacenter_combobox.get()
        if not text:
            # 如果输入为空，显示所有选项
            self.datacenter_combobox['values'] = self.all_datacenters
            return
        
        # 筛选匹配的数据中心
        # 数据中心格式为 "XXX - 地点"，我们需要获取破折号前的缩写部分
        filtered = ["全部"]
        for dc in self.all_datacenters[1:]:  # 跳过"全部"选项
            datacenter_code = dc.split(' - ')[0].strip()
            # 如果数据中心代码的首字母（忽略大小写）匹配用户输入
            if datacenter_code and datacenter_code[0].lower() == text[0].lower():
                filtered.append(dc)
        
        # 更新下拉列表的值
        self.datacenter_combobox['values'] = filtered
        
        # 保持下拉列表打开
        self.datacenter_combobox.event_generate('<Down>')