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



def get_store_orders(access_token, target_store):
    """
    获取指定店铺的完整订单信息（按小时分段请求避免分页限制）
    参数：
    access_token - 有赞API访问令牌
    target_store - 要筛选的目标店铺名称
    """
    url = "https://open.youzanyun.com/api/youzan.trades.sold.get/4.0.4"
    headers = {"Content-Type": "application/json"}
    all_orders = []  # 存储所有订单

    # 获取当天日期
    today = datetime.now().strftime("%Y-%m-%d")
    print(today)
    # today ="2025-07-07"

    try: 
        for hour in range(23):
            # 计算当前小时的开始和结束时间
            start_time = f"{today} {hour:02d}:00:00"
            # start_time = f"2024-06-08 {hour:02d}:00:00"
            end_hour = (hour + 1) % 24
            end_date = today if end_hour != 0 else (datetime.now() + timedelta(days=1)).strftime("%Y-%m-%d")
            end_time = f"{end_date} {end_hour:02d}:00:00"

            next_cursor = None  # 初始化分页游标
            has_next_page = True

            # 当前小时段的分页循环
            while has_next_page:
                params = {
                    "page_size": "100",
                    "start_success": start_time,
                    "end_success": end_time
                }

                # 添加分页游标参数
                if next_cursor:
                    params["cursor"] = next_cursor

                response = requests.post(
                    f"{url}?access_token={access_token}",
                    json=params,
                    headers=headers
                )
                response.raise_for_status()
                result = response.json()

                if not result.get("success"):
                    print(str(result))
                    print(f"API请求失败: {result.get('message')}")
                    break

                # 处理当前页数据
                if "data" in result and "full_order_info_list" in result["data"]:
                    for order in result["data"]["full_order_info_list"]:
                        order_info = order["full_order_info"]["order_info"]
                        if order_info.get("shop_name") == target_store:
                            all_orders.append(order["full_order_info"])

                # 检查分页信息
                if "data" in result and "paginator" in result["data"]:
                    has_next_page = result["data"]["paginator"].get("has_next", False)
                    next_cursor = result["data"]["paginator"].get("next_cursor")
                else:
                    has_next_page = False
        # print(all_orders)

        return all_orders

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
        if "MARK_PAY_EXCHANGE" in str(order["order_info"]["pay_type_str"]):
            # print(order)
            # print("MARK_PAY_EXCHANGE")
            continue
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
def price_change_free(order_list, access_token):
    """
    根据订单列表中的tid查询并更新payment金额
    参数：
    order_list - 包含订单信息的列表，每个订单需有'tid'和'payment'字段
    access_token - 有赞API访问令牌
    """
    url = "https://open.youzanyun.com/api/youzan.cardvoucher.valuecard.pay.rcd.bypub.search/3.0.1"
    headers = {"Content-Type": "application/json"}
    
    for order in order_list:
        tid = order.get('tid')
        if not tid:
            print("订单缺少tid，跳过处理")
            continue
            
        # 构造请求参数
        params = {
            "page_size": 50,
            "page": 1,
            "trade_no": tid
        }
        
        try:
            # 发送POST请求
            response = requests.post(
                f"{url}?access_token={access_token}",
                json=params,
                headers=headers
            )
            response.raise_for_status()
            result = response.json()
            
            # 检查API响应是否成功
            if not result.get("success", False):
                print(f"查询订单 {tid} 失败: {result.get('message', '未知错误')}")
                continue
                
            # 提取bonus_pay_amount（从items数组中获取第一个元素的bonus_pay_amount）
            items = result.get("data", {}).get("items", [])
            bonus_pay_amount = items[0].get("bonus_pay_amount", 0) if items else 0
            
            # 计算调整后的payment
            if bonus_pay_amount > 0:
                try:
                    payment = float(order.get("payment", "0.00"))
                    adjusted_bonus = bonus_pay_amount / 100
                    new_payment = payment - adjusted_bonus
                    order["payment"] = f"{new_payment:.2f}"
                    print(f"订单 {tid} 已更新，调整金额: {adjusted_bonus}")
                except ValueError:
                    print(f"订单 {tid} payment格式错误: {order.get('payment')}")
                    
        except Exception as e:
            print(f"处理订单 {tid} 时发生异常: {str(e)}")
    
    return order_list


# 使用示例
if __name__ == "__main__":
    # 获取访问令牌和订单数据（接前文代码）
    token = get_youzan_access_token()
    target_shop = "上生新所门店"
    orders = get_store_orders(token, target_shop)
    # print(orders)
    # 提取关键字段
    extracted_orders = extract_order_details(orders)
    # print(extracted_orders)
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
    all_price=0
    print("开始")
    print(len(data))
    for i in data:
        print(i)
        all_price = all_price + float(i['payment'])
    print(all_price)