
import hashlib
import json,time

def get_sign(app_secret, params, sign_method='MD5'):
    """
    生成签名
    :param app_secret: 应用密钥
    :param params: 参数字典
    :param sign_method: 签名方法，默认为MD5
    :return: 签名结果字符串
    """
    # 1. 排除空值参数和sign参数，并按照参数名ASCII码从小到大排序（字典序）
    sorted_params = sorted([(k, v) for k, v in params.items() if k != 'sign' and v is not None])
    
    # 2. 拼接成key1=value1&key2=value2的格式
    stringA = '&'.join([f"{k}={v}" for k, v in sorted_params])
    
    # 3. 在stringA最后拼接上&key=API密钥的值
    stringSignTemp = f"{stringA}&key={app_secret}"
    
    # 4. 根据签名方法生成签名并转换为大写
    if sign_method.upper() == 'MD5':
        m = hashlib.md5()
        m.update(stringSignTemp.encode('utf-8'))
        signValue = m.hexdigest().upper()
    else:
        # 可以根据需要添加其他签名方法
        raise ValueError(f"不支持的签名方法: {sign_method}")
    
    return signValue



if __name__ == '__main__':
    params = {}
    params['method'] = 'gogo.open.auto.routing'
    params['timestamp'] = time.strftime('%Y%m%d%H%M%S', time.localtime())
    params['messageFormat'] = 'json'
    params['appKey'] = '2c968875814106ca0181adf9657d0005'
    params['v'] = '1.0'
    params['signMethod'] = 'MD5'
    params['lowerMethod'] = 'com.gooagoo.exportbill'
    params['appId'] = "d1667ebbaa3e4935a7e09be1a50f0af5"

    map_data = {}
    map_data['terminalNumber'] = '6A53BB2D7CDE'
    map_data['saleTime'] = '2025-09-12 00:00:00'
    map_data['billType'] = '3'
    map_data['exactBillType'] = '3'
    map_data['billSerialNumber'] = 'test001'
    map_data['thirdPartyOrderNo'] = 'test001'
    map_data['totalNum'] = 1
    map_data['totalFee'] = 0.01
    map_data['paidAmount'] = 0.01
    map_data['receivableAmount'] = 0.01

    params['data'] = json.dumps(map_data, ensure_ascii=False)

    # 生成签名
    app_secret = 'D22B58F177D6739D413C5FE24CD32ED0'
    sign = get_sign(app_secret, params)

    print(f"生成的签名: {sign}")
    params["sign"] = sign
    print(params)
    print(time.strftime('%Y%m%d%H%M%S', time.localtime()))