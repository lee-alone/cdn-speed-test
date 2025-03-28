import configparser
import os

def load_config():
    config = configparser.ConfigParser()
    config.read('config.ini')
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
    filepath = config['DEFAULT'].get('filepath', '')
    if not filepath:
        # 如果config.ini中不存在filepath，则创建find.txt文件，并将路径写入config.ini
        current_path = os.path.dirname(os.path.abspath(__file__))
        find_path = os.path.join(current_path, 'find.txt')
        with open(find_path, 'w', encoding='utf-8') as f:
            f.write(current_path)
        config['DEFAULT']['filepath'] = find_path
        save_config(config, config['DEFAULT']['expected_servers'], config['DEFAULT']['use_tls'], config['DEFAULT']['ip_type'], config['DEFAULT']['bandwidth'], config['DEFAULT']['timeout'], config['DEFAULT']['download_time'], find_path)
        return find_path
    else:
        return filepath