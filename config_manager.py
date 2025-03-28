import configparser
import os

def load_config():
    config = configparser.ConfigParser()
    try:
        config.read('config.ini')
    except FileNotFoundError:
        # 如果config.ini文件不存在，则创建该文件并写入默认值
        config['DEFAULT'] = {
            'expected_servers': '1',
            'use_tls': 'False',
            'ip_type': 'ipv4',
            'bandwidth': '5',
            'timeout': '5',
            'download_time': '15',
            'filepath': './'
        }
        with open('config.ini', 'w', encoding='utf-8') as configfile:
            config.write(configfile)
    return config

def save_config(config, expected_servers, use_tls, ip_type, bandwidth, timeout, download_time, filepath):
    config['DEFAULT']['expected_servers'] = str(expected_servers)
    config['DEFAULT']['use_tls'] = str(use_tls)
    config['DEFAULT']['ip_type'] = ip_type
    config['DEFAULT']['bandwidth'] = str(bandwidth)
    config['DEFAULT']['timeout'] = str(timeout)
    config['DEFAULT']['download_time'] = str(download_time)
    config['DEFAULT']['filepath'] = filepath
    with open('config.ini', 'w', encoding='utf-8') as configfile:
        config.write(configfile)

def load_find_path():
    config = load_config()
    filepath = config['DEFAULT'].get('filepath', './')
    if filepath == './':
        current_path = os.path.dirname(os.path.abspath(__file__))
        config['DEFAULT']['filepath'] = current_path
        save_config(config, config['DEFAULT']['expected_servers'], config['DEFAULT']['use_tls'], config['DEFAULT']['ip_type'], config['DEFAULT']['bandwidth'], config['DEFAULT']['timeout'], config['DEFAULT']['download_time'], current_path)
        return current_path
    else:
        return filepath