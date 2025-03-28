import time
import os
from typing import Optional, List
from dataclasses import dataclass
from config_manager import load_find_path
import typing
import re

if typing.TYPE_CHECKING:
    from gui import SpeedTestResult

def save_results(qualified_servers: Optional[List['SpeedTestResult']] = None, expected_bandwidth = None, find_path = None, use_tls = None, TLS_PORTS = None, NON_TLS_PORTS = None):
    if qualified_servers is None:
        return

    if not qualified_servers:
        print("没有找到符合要求的服务器")
        return
        
    now = time.strftime("%Y-%m-%d %H:%M:%S", time.localtime())
    filepath = os.path.join(find_path, 'find.txt')
    try:
        with open(filepath, 'a', encoding='utf-8') as f:
            f.write(f"[{now}] 符合要求的服务器:\n")
            f.write(f"期望带宽: {expected_bandwidth}Mbps\n")
            for result in qualified_servers:
                # 检查IP是否已经存在
                if _is_ip_exists(filepath, result.ip):
                    print(f"IP {result.ip} 已经存在，跳过写入")
                    continue
                _write_result_to_file(f, result)
                ports = TLS_PORTS if use_tls else NON_TLS_PORTS
                f.write(f"支持端口: {ports}\n")
                f.write("-" * 20 + "\n")
        #  print(f"已将{len(qualified_servers)}个符合要求的服务器信息保存到 {filepath}")
    except Exception as e:
        print(f"保存结果失败: {str(e)}")

def _is_ip_exists(filepath: str, ip: str) -> bool:
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            content = f.read()
            # 使用正则表达式匹配IP地址，避免误判
            pattern = re.compile(r"IP: " + re.escape(ip))
            return bool(pattern.search(content))
    except FileNotFoundError:
        return False
    except Exception as e:
        print(f"检查IP是否存在时发生错误: {str(e)}")
        return False

def _read_colo_mapping(colo_file: str) -> dict:
    colo_mapping = {}
    try:
        with open(colo_file, 'r', encoding='utf-8') as f:
            for line in f:
                parts = line.strip().split(',')
                if len(parts) == 2:
                    city_region = parts[0]
                    code = parts[1].split('(')[-1].split(')')[0]
                    colo_mapping[code] = f"{city_region},{parts[1]}"
    except FileNotFoundError:
        print(f"文件未找到: {colo_file}")
    except Exception as e:
        print(f"读取文件时发生错误: {colo_file}: {str(e)}")
    return colo_mapping

def _write_result_to_file(f, result: 'SpeedTestResult'):
    f.write(f"IP: {result.ip}\n")
    f.write(f"延迟: {result.latency}ms\n")
    f.write(f"平均速度: {result.speed}Mbps\n")
    f.write(f"峰值速度: {float(result.peak_speed):.2f}Mbps\n")
    
    colo_mapping = _read_colo_mapping('colo.txt')
    datacenter = result.datacenter
    if datacenter in colo_mapping:
        f.write(f"数据中心: {colo_mapping[datacenter]}\n")
    else:
        f.write(f"数据中心: {datacenter}\n")