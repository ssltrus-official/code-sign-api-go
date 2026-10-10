# HTTP API 文档

[api.swagger.yaml](api.swagger.yaml) 为 Swagger 2.0 格式，
可导入 Swagger UI、Swagger Editor 或 Apifox 查看接口、请求和响应。

使用 HTTP Basic Auth 时，用户名填写 Access Key，密码填写 Access Secret。
版本查询接口无需认证；其他接口需要认证。实际服务地址由交付方提供。

通过 `code` 判断业务结果：0 表示成功，非零表示失败，HTTP 200 不一定表示业务成功。
`remoteIp`、`accessKey`、`userAgent` 为响应中的只读信息，无需在签名请求中填写。

原始哈希签名接口支持 SHA1、SHA256、SHA384、SHA512，填充方式为 PKCS1。
实际在文档工具中调用签名接口会消耗签名额度；超时后不要盲目重试。
