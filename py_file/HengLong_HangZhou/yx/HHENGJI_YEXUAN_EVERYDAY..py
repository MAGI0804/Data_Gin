
import requests
from datetime import datetime, timedelta
import os
import hmac
import base64
import hashlib
import json
import urllib.parse

from yingye import get_yingye

url = "https://wlkpos.hanglung.com.cn:8280/HLD/salestrans.asmx"
# url = "https://plazapo.hanglung.com.cn:8280/HLD/salestrans.asmx"
# username = 40001
username =""
# password ="Hk29bv"
password=""

# storecode = 40001
storecode = 416101

# tillid = "1"
tillid = "01"

# mallitemcode = 40001
mallitemcode = "E6600000074"
licensekey = ""
mallid = "WESTLAKE66"
cashier = storecode
# plucode = 40001
plucode = mallitemcode
openId = "640a921c8e6d225b386be125cdcd52e3"
grantCode = "KT8IyBvygA"
open_key = 'VcF3zUcMoa546e1a438c6b30d32ec6f5074a113566ebS6262A'
nonce = "11886"
shopCode = "YX006"
# print(get_yingye(openId,grantCode,open_key,nonce,shopCode))
pay_amount = get_yingye(openId,grantCode,open_key,nonce,shopCode)

pay_amount=0.01
tid = "test000319"

# 获取当前日期的前一天，格式为yyyymmdd
today = datetime.now()
yesterday = today - timedelta(days=1)
day_ago = yesterday.strftime("%Y%m%d")
# tid ="YX006"+day_ago
print(day_ago)
print(tid)
time_ago = "233000"

post_xml = f"""
<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <postsalescreate xmlns="http://tempurl.org" xmlns:i="http://www.w3.org/2001/XMLSchema-instance">
      <astr_request>
        <header>
          <licensekey>{licensekey}</licensekey>
          <username>{username}</username>
          <password>{password}</password>
          <pagerecords>0</pagerecords>
          <pageno>0</pageno>
          <updatecount>0</updatecount>
          <messagetype>SALESDATA</messagetype>
          <messageid>332</messageid>
          <version>V332M</version>
        </header>
        <salestotal>
          <localstorecode>{storecode}</localstorecode>
          <reservedocno></reservedocno>
          <txdate_yyyymmdd>{day_ago}</txdate_yyyymmdd>
          <txtime_hhmmss>{time_ago}</txtime_hhmmss>
          <mallid>{mallid}</mallid>
          <storecode>{storecode}</storecode>
          <tillid>{tillid}</tillid>
          <salestype>SA</salestype>
          <txdocno>{tid}</txdocno>
          <orgtxdate_yyyymmdd></orgtxdate_yyyymmdd>
          <orgstorecode></orgstorecode>
          <orgtillid></orgtillid>0
          <txorgdocno></txorgdocno>
          <mallitemcode>{mallitemcode}</mallitemcode>
          <cashier>{cashier}</cashier>
          <netqty>1</netqty>
          <originalamount>0</originalamount>
          <sellingamount>{pay_amount}</sellingamount>
          <couponqty>0</couponqty>
          <totaldiscount>
          </totaldiscount>
          <ttltaxamount1>0</ttltaxamount1>
          <ttltaxamount2>0</ttltaxamount2>
          <netamount>{pay_amount}</netamount>
          <paidamount>{pay_amount}</paidamount>
          <changeamount>0</changeamount>
          <priceincludetax></priceincludetax>
          <issueby>000</issueby>
          <issuedate_yyyymmdd>{day_ago}</issuedate_yyyymmdd>
          <issuetime_hhmmss>{time_ago}</issuetime_hhmmss>
        </salestotal>
        <salesitems>
          <salesitem>
            <iscounteritemcode>1</iscounteritemcode>
            <lineno>1</lineno>
            <storecode>{storecode}</storecode>
            <mallitemcode>{mallitemcode}</mallitemcode>
            <counteritemcode>{plucode}</counteritemcode>
            <itemcode>{plucode}</itemcode>
            <plucode>{plucode}</plucode>
            <invttype>1</invttype>
            <qty>1</qty>
            <exstk2sales>1</exstk2sales>
            <originalprice>0</originalprice>
            <sellingprice>0</sellingprice>
            <vipdiscountpercent>0</vipdiscountpercent>
            <vipdiscountless>0</vipdiscountless>
            <totaldiscountless1>0</totaldiscountless1>
            <totaldiscountless2>0</totaldiscountless2>
            <totaldiscountless>0</totaldiscountless>
            <netamount>{pay_amount}</netamount>
            <bonusearn>0</bonusearn>
          </salesitem>
        </salesitems>
        <salestenders>
          <salestender>
            <lineno>1</lineno>
            <tendercode>CH</tendercode>
            <tendertype>0</tendertype>
            <tendercategory>0</tendercategory>
            <payamount>{pay_amount}</payamount>
            <baseamount>{pay_amount}</baseamount>
            <excessamount>0</excessamount>
          </salestender>
        </salestenders>
        <salesdelivery>
        </salesdelivery>
      </astr_request>
    </postsalescreate>
  </soap:Body>
</soap:Envelope>
"""

# 日志功能
def write_log(message, log_dir=r"C:\Users\易理志\Desktop\每日推送\HengLong_HangZhou\yx\log"):
    if not os.path.exists(log_dir):
        os.makedirs(log_dir)
    log_file = os.path.join(log_dir, f"sales_log_{datetime.now().strftime('%Y%m%d')}.log")
    timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
    with open(log_file, 'a', encoding='utf-8') as f:
        f.write(f"[{timestamp}] {message}\n")

# 发送钉钉消息
def send_dingtalk_message(message, webhook_token="b9fec592d810a157727b65b234b855548ff867db391802917cb06c9acd9beb36",
                          sign_secret="SEC16c77ae990ef8c087e105de0a65251775a2760cfb7c5f5f20ecb684b1f32e6ad"):
    webhook_url = f"https://oapi.dingtalk.com/robot/send?access_token={webhook_token}"
    timestamp = str(round(datetime.now().timestamp() * 1000))
    secret_enc = sign_secret.encode('utf-8')
    string_to_sign = f'{timestamp}\n{sign_secret}'
    string_to_sign_enc = string_to_sign.encode('utf-8')
    hmac_code = hmac.new(secret_enc, string_to_sign_enc, digestmod=hashlib.sha256).digest()
    sign = urllib.parse.quote_plus(base64.b64encode(hmac_code))
    url = f"{webhook_url}&timestamp={timestamp}&sign={sign}"
    headers = {'Content-Type': 'application/json'}
    data = {
        "msgtype": "text",
        "text": {
            "content": message
        }
    }
    try:
        response = requests.post(url, headers=headers, data=json.dumps(data))
        return response.json()
    except Exception as e:
        return {"errcode": -1, "errmsg": str(e)}

# 发送POST请求的方法
def send_post_request(post_xml):
    headers = {
        'Content-Type': 'text/xml; charset=utf-8',
        'SOAPAction': 'http://tempurl.org/postsalescreate'
    }
    response = requests.post(url, data=post_xml.encode('utf-8'), headers=headers)
    return response.text


if __name__ == '__main__':
    write_log(f"开始处理销售数据，日期: {day_ago}")
    write_log(f"交易ID: {tid}, 金额: {pay_amount}")
    
    try:
        response = send_post_request(post_xml)
        write_log(f"发送POST请求成功，响应内容: {response}")
        
        if "responsecode>0<" in response or "上传成功" in response:
            write_log("销售数据上传成功")
            dingtalk_msg = f"销售数据上传成功\n日期: {day_ago}\n交易ID: {tid}\n金额: {pay_amount}"
            send_dingtalk_message(dingtalk_msg)
            write_log(f"钉钉消息发送成功: {dingtalk_msg}")
        else:
            write_log(f"销售数据上传失败，响应: {response}")
            dingtalk_msg = f"销售数据上传失败\n日期: {day_ago}\n交易ID: {tid}\n响应: {response}"
            send_dingtalk_message(dingtalk_msg)
    except Exception as e:
        error_msg = f"处理销售数据时发生错误: {str(e)}"
        write_log(error_msg)
        send_dingtalk_message(error_msg)