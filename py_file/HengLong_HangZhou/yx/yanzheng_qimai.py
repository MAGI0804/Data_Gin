import hmac
import hashlib
import base64
import urllib.parse
import time

def k_sort(params):
    # 对参数字典的键进行字典序排序
    sorted_keys = sorted(params.keys())
    # 构建键值对字符串
    param_strs = []
    for key in sorted_keys:
        param_strs.append(f"{key}={params[key]}")
    # 用&连接所有键值对
    joined_str = '&'.join(param_strs)
    # 对整个字符串进行URL编码
    encoded_str = urllib.parse.quote(joined_str)
    # 替换特定字符（Java代码中的特殊处理）
    encoded_str = encoded_str.replace('%3D', '=').replace('%26', '&')
    return encoded_str

def compute_signature(base_string, key_string):
    # 将密钥和签名文本转换为字节
    key_bytes = key_string.encode('utf-8')
    message_bytes = base_string.encode('utf-8')
    # 创建HmacSHA1签名
    hmac_obj = hmac.new(key_bytes, message_bytes, hashlib.sha1)
    # 获取签名的二进制数据并进行Base64编码，去除末尾换行符
    signature_bytes = hmac_obj.digest()
    base64_signature = base64.b64encode(signature_bytes).decode('utf-8').strip()
    return base64_signature

def generate_token(params, secret_key):
    # 1. 对参数进行kSort处理
    signature_text = k_sort(params)
    
    # 2. 使用HmacSHA1算法签名并进行Base64编码
    base64_signature = compute_signature(signature_text, secret_key)
    
    # 3. 对Base64编码后的字符串进行URL编码
    token = urllib.parse.quote(base64_signature)
    
    return token

# 示例用法 - 匹配Java代码中的示例
if __name__ == '__main__':

    params = {
        'openId': '640a921c8e6d225b386be125cdcd52e3',
        'grantCode': 'KT8IyBvygA',
        'timestamp': int(time.time()),
        'nonce': '11886'
    }
    print(params)

    open_key = 'VcF3zUcMoa546e1a438c6b30d32ec6f5074a113566ebS6262A'
    
    # 1. 执行kSort操作
    key_sort_string = k_sort(params)
    print(f'kSort结果: {key_sort_string}')
    
    # 2. 计算签名
    token_before = compute_signature(key_sort_string, open_key)
    print(f'签名结果(Base64): {token_before}')
    
    # 3. 生成最终token
    token = generate_token(params, open_key)
    print(f'最终Token: {token}')