from datetime import datetime, timedelta
import time
import requests
from yanzheng_qimai import generate_token,compute_signature,k_sort

def get_yingye(openId,grantCode,open_key,nonce,shopCode):
    url = "https://openapi.qmai.cn/v3/dataone/finance/summary/businessRecord"
    timestamp =int(time.time())
    param = {
        'openId': openId,
        'grantCode': grantCode,
        'timestamp': timestamp,
        'nonce': '11886'
    }

    # 1. 执行kSort操作
    key_sort_string = k_sort(param)
    # print(f'kSort结果: {key_sort_string}')

    # 2. 计算签名
    token_before = compute_signature(key_sort_string, open_key)

    # 3. 生成最终token
    token = generate_token(param, open_key)
    # print(f'最终Token: {token}')
    today = (datetime.now() - timedelta(days=1)).strftime("%Y-%m-%d")

    # today ="2025-09-28"
    body ={
        "openId":openId,
        "grantCode":grantCode,
        "nonce":nonce,
        "timestamp":timestamp,
        "token":token,
        "params":{
            "end_date": today,
            "start_date": today,
            "shopCode": shopCode
        }
    }
    headers = {
        "Content-Type": "application/json"
    }
    print(body)
    try:
        response = requests.post(url, headers=headers, json=body)
        response.raise_for_status()  # 抛出HTTP错误
        print(response.json())
        return response.json()["data"]["resultList"][0]["income"]
    except requests.exceptions.RequestException as e:
        print(f"请求发送失败: {str(e)}")
        return None


if __name__ == '__main__':
    openId = "640a921c8e6d225b386be125cdcd52e3"
    grantCode = "KT8IyBvygA"
    open_key = 'VcF3zUcMoa546e1a438c6b30d32ec6f5074a113566ebS6262A'
    nonce = "11886"
    shopCode = "YX006"
    print(get_yingye(openId,grantCode,open_key,nonce,shopCode))