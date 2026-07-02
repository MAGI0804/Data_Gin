import json
from datetime import datetime, timedelta
import requests


def get_youzan_access_token():
    """获取有赞open平台access_token"""
    url = "https://open.youzanyun.com/auth/token"
    headers = {"Content-Type": "application/json"}
    data = {
        "authorize_type": "silent",
        "client_id": "379981eff640bbb278",
        "client_secret": "1ef6d04d42b03784bd75fc1b74493c06",
        "grant_id": "15707004",
        "refresh": False
    }

    try:
        response = requests.post(url, json=data, headers=headers)
        response.raise_for_status()
        result = response.json()

        if result.get("success") and result.get("code") == 200:
            return result["data"]["access_token"]
        print(f"获取access_token失败: {result.get('message')}")
        return None
    except Exception as e:
        print(f"请求异常: {str(e)}")
        return None


def free_price_request(token:str,target_kdt_id:int):
    url = "https://open.youzanyun.com/api/youzan.trade.refund.search/3.0.1"
    headers = {"Content-Type": "application/json"}
    today = datetime.now().strftime("%Y-%m-%d")
    print(today)
    # print("123")
    # today = "2025-09-24"
    # today =(datetime.now() - timedelta(days=1)).strftime("%Y-%m-%d")
    print(today)
    params = {
    "sale_way":"1",
    "node_kdt_id":target_kdt_id
    }


    try:
        response = requests.post(
            f"{url}?access_token={token}",
            json=params,
            headers=headers
        )
        response.raise_for_status()
        result = response.json()
        print(result)

        if not result.get("success"):
            print(f"API请求失败: {result.get('message')}")
            return []
        # print(result)

        # 筛选目标店铺订单
        filtered_orders = []
        for order in result["data"]["refunds"]:
            # print(order)
            if today in order["modified"]:
                filtered_orders.append(order)
        print(filtered_orders)
        return filtered_orders
    except Exception as e:
        print(f"订单获取异常: {str(e)}")
        return []





def extract_order_details(full_order_list):
    
    """
    从完整订单数据中提取关键交易字段
    参数：
    full_order_list - 完整订单列表（get_store_orders返回的结果）

    返回：
    包含以下字段的字典列表：
    cashier_id, tid, platform, pay_end_time,
    payment, total_fee, created_time, success_time,
    is_refund, totalAmt
    """
    extracted_data = []

    for order in full_order_list:
        try:
            # 基础字段提取
            order_info = order["order_info"]
            pay_info = order["pay_info"]
            source_info = order["source_info"]

            # 处理收银员ID（可能不存在）
            cashier_id = order_info.get("order_extra", {}).get("cashier_id", "")

            # 处理支付结束时间（取最后一期支付时间）
            phase_payments = pay_info.get("phase_payments", [])
            pay_end_time = phase_payments[-1]["pay_end_time"] if phase_payments else ""

            # 处理is_refund和totalAmt
            is_refund_flag = order_info.get("order_tags", {}).get("is_refund", False)
            payment_str = pay_info.get("payment", "0.00")

            # 转换payment为浮点数并计算totalAmt
            try:
                payment_float = float(payment_str)
            except ValueError:
                payment_float = 0.00  # 默认值处理异常情况
            total_amt = -payment_float if is_refund_flag else payment_float
            total_amt_str = "{:.2f}".format(total_amt)

            # 确定is_refund状态
            is_refund_value = "ONLINEREFUND" if is_refund_flag else "SALE"

            # 构造数据记录
            record = {
                "cashier_id": cashier_id,
                "tid": order_info.get("tid", ""),
                "platform": source_info.get("source", {}).get("platform", ""),
                "pay_end_time": pay_end_time,
                "payment": payment_str,
                "total_fee": pay_info.get("total_fee", "0.00"),
                "created_time": order_info.get("created", ""),
                "success_time": order_info.get("success_time", ""),
                "is_refund": is_refund_value,
                "totalAmt": total_amt_str
            }

            extracted_data.append(record)

        except KeyError as e:
            print(f"字段解析异常，缺失关键字段：{str(e)}")
            continue
        except IndexError as e:
            print(f"支付时间解析异常：{str(e)}")
            continue

    return extracted_data


# 使用示例
if __name__ == "__main__":
    # 获取访问令牌和订单数据（接前文代码）
    token = get_youzan_access_token()
    print(token)
    target_shop = "上生新所门店"
    orders = get_store_orders(token, target_shop)
    print(orders)
    # 提取关键字段
    extracted_orders = extract_order_details(orders)
    print(extracted_orders)
    # 打印结果示例
    # dindan_neiron=json.dumps(extracted_orders[:2], indent=2, ensure_ascii=False)
    dindan_neiron=extracted_orders
    print(dindan_neiron)
    # data = json.loads(dindan_neiron)
    data = dindan_neiron
    # print(data[0]['cashier_id'])
    # print(dindan_neiron[1])
    # print(data[0]["success_time"])
    # print(datetime.strptime(data[0]["success_time"], "%Y-%m-%d %H:%M:%S").strftime("%Y%m%d%H%M%S"))
    # for i in data:
    #     print(i)
    target_kdt_id = 75095302
    free_price = free_price_request(token,target_kdt_id)
    for i in free_price:
        add_dict = {'cashier_id': '21700647496', 'tid': i["refund_id"], 'platform': 'other', 'pay_end_time': '', 'payment': "-"+i["refund_fee"], 'total_fee': "-"+i["refund_fee"], 'created_time': '2025-05-29 11:39:43', 'success_time': i["modified"], 'is_refund': 'ONLINEREFUND', 'totalAmt': "-"+i["refund_fee"]}
        data.append(add_dict)
    for i in data:
        print(i)
    