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

### POST /charge/v1/coupons

创建一个优惠券

Body Parameters:
```
kind: 优惠券种类
expire_on: 到期时间
amount: 优惠券金额
```
eg:
```
POST /charge/v1/coupons HTTP/1.1
Accept: application/json 
Content-Type: application/json 
Authorization: Bearer XXXXXXXXXXXXXXXXXXXXXXX
{
    "kind": "recharge",
    "expire_on": "2017-01-01T00:00:00Z",
    "amount": 68
}
```

Return Result (json):
```
code: 返回码
msg: 返回信息
data.serial: 优惠券序列号
data.code: 优惠码
```

### DELETE /charge/v1/coupons/{serial}

删除一个优惠券，并不是真的把优惠券从表中删除，而是把优惠券的 'status' 置为 'unavailable'。

Path Parameters:
```
serial: 优惠券序列号
```
eg:
```
DELETE /charge/v1/coupons/XXXXXXX HTTP/1.1
Accept: application/json 
Content-Type: application/json 
Authorization: Bearer XXXXXXXXXXXXXXXXXXXXXXX
```

Return Result (json):

```
code: 返回码
msg: 返回信息
```

### GET /charge/v1/coupons/{serial}

查询一个优惠券

Path Parameters:
```
serial: 优惠券序列号
```
eg:
```
GET /charge/v1/coupons/XXXXXXX HTTP/1.1
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

### GET /charge/v1/coupons

查询优惠券列表

eg:
```
GET /charge/v1/coupons HTTP/1.1
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

### PUT /charge/v1/coupons/use/{serial}

使用一个优惠券

Path Parameters:
```
serial: 优惠券序列号
```

Body Parameters:
```
code: 优惠码
namespace: 充值区域
```
eg:
```
PUT /charge/v1/coupons/XXXXXXXXXXXXXXXX HTTP/1.1
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

