# datafoundry_coupon

```
datafoundry优惠券微服务
```

##数据库设计

```
CREATE TABLE IF NOT EXISTS DF_COUPON
(
    ID                BIGINT NOT NULL AUTO_INCREMENT,
    SERIAL            VARCHAR(64) NOT NULL,
    CODE              VARCHAR(64) NOT NULL,
    KIND              VARCHAR(32) NOT NULL,
    EXPIRE_ON         DATETIME NOT NULL,
    AMOUNT            DOUBLE(10,2) NOT NULL,
    CREATE_AT         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UPDATE_AT         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    USE_TIME          DATETIME,
    USERNAME          VARCHAR(32),
    NAMESPACE         VARCHAR(64),
    STATUS            VARCHAR(32),
    PRIMARY KEY (ID)
) DEFAULT CHARSET=UTF8;
```

## API设计

### POST /charge/v1/coupons?region={region}

创建一个优惠券

Path Parameters:
```
region: 区域，分别是一区和二区
```

Body Parameters:
```
kind: 优惠券种类
expire_on: 多少天后过期
amount: 优惠券金额
```
eg:
```
POST /charge/v1/coupons?region=cn-north-1 HTTP/1.1
Accept: application/json 
Content-Type: application/json 
Authorization: Bearer XXXXXXXXXXXXXXXXXXXXXXX
{
    "kind": "recharge",
    "expire_on": 30,
    "amount": 68
}
```

Return Result (json):
```
code: 返回码
msg: 返回信息
data.serial: 优惠券序列号
data.code: 优惠码
data.expire_on: 过期时间
data.amount: 充值卡金额
```

### DELETE /charge/v1/coupons/{serial}?region={region}

删除一个优惠券，并不是真的把优惠券从表中删除，而是把优惠券的 'status' 置为 'unavailable'。

Path Parameters:
```
serial: 优惠券序列号
region: 区域，分别是一区和二区
```
eg:
```
DELETE /charge/v1/coupons/XXXXXXX?region=cn-north-1 HTTP/1.1
Accept: application/json 
Content-Type: application/json 
Authorization: Bearer XXXXXXXXXXXXXXXXXXXXXXX
```

Return Result (json):

```
code: 返回码
msg: 返回信息
```

### GET /charge/v1/coupons/{code}?region={region}

查询一个优惠券

Path Parameters:
```
code: 充值码
region: 区域，分别是一区和二区
```
eg:
```
GET /charge/v1/coupons/XXXXXXX?region=cn-north-1 HTTP/1.1
Accept: application/json 
Content-Type: application/json 
Authorization: Bearer XXXXXXXXXXXXXXXXXXXXXXX
```

Return Result (json):
```
code: 返回码
msg: 返回信息
data.serial: 优惠券序列号
data.amount: 优惠券金额
data.expire_on: 到期时间
data.status: 优惠券状态
```

### GET /charge/v1/coupons?region={region}&page={page}&size={size}

查询优惠券列表

Path Parameters:
```
region: 区域，分别是一区和二区
page: 页码
size: 一页的大小
```

eg:
```
GET /charge/v1/coupons?region=cn-north-1&page=1&size=50 HTTP/1.1
Accept: application/json 
Content-Type: application/json 
Authorization: Bearer XXXXXXXXXXXXXXXXXXXXXXX
```

Return Result (json):
```
code: 返回码
msg: 返回信息
data.total
data.results
data.results[0].serial: 优惠券序列号
data.results[0].amount: 优惠券金额
data.results[0].expire_on: 到期时间
data.results[0].status: 优惠券状态
...
```

### PUT /charge/v1/coupons/use/{serial}？region={region}

使用一个优惠券

Path Parameters:
```
serial: 优惠券序列号
region: 区域，分别是一区和二区
```

Body Parameters:
```
code: 优惠码
namespace: 充值区域
```
eg:
```
PUT /charge/v1/coupons/XXXXXXXXXXXXXXXX?region=cn-north-1 HTTP/1.1
Accept: application/json 
Content-Type: application/json 
Authorization: Bearer XXXXXXXXXXXXXXXXXXXXXXX

{
    "code": "XXXXXXXXXXXXXXXXXX",
    "namespace": "test"
}
```

Return Result (json):
```
code: 返回码
msg: 返回信息
data.amount: 充值金额
data.namespace: 充值区域
```

### POST /charge/v1/provide/coupons

微信扫描公众号提供一个充值码

Body Parameters:
```
openId: 扫描微信的唯一标识
provideTime: 提供时间的时间戳
```
eg:
```
PUT /charge/v1/provide/coupons HTTP/1.1
Accept: application/json 
Content-Type: application/json 
Authorization: Bearer XXXXXXXXXXXXXXXXXXXXXXX

{
    "openId": "XXXXXXXXXXXXXXXXXX",
    "provideTime": 1483584876
}
```

Return Result (json):
```
code: 返回码
msg: 返回信息
data.isProvide: 是否已经提供过
data.code: 充值码
```
