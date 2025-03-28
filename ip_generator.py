import ipaddress
import random
import logging
from typing import Optional

logger = logging.getLogger(__name__)

class IPGenerator:
    def __init__(self):
        self.used_ips = set()

    def generate_ip(self, subnet: str, ip_type: str) -> Optional[str]:
        try:
            if ip_type == "ipv4":
                network = ipaddress.IPv4Network(subnet)
            elif ip_type == "ipv6":
                network = ipaddress.IPv6Network(subnet)
            else:
                raise ValueError("Invalid IP type")

            if network.num_addresses <= 2:
                return None

            if ip_type == "ipv4":
                ip = str(random.choice(list(network.hosts())))
            else:  # ipv6
                ip = str(network[random.randint(1, network.num_addresses - 1)])

            if ip not in self.used_ips:
                self.used_ips.add(ip)
                return ip
            return None
        except Exception as e:
            logger.error(f"生成 IP 失败: {str(e)}")
            return None