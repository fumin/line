# Lineye, Line眼, Line信息保障系统

✅ 24/7全时段、实时低延迟监控

✅ 多模态信息处理，覆盖音视频多语言文字信息

✅ 高稳定存储架构，支援 10个9以上多资料中心信息存储

✅ 硬严苛安全设计，贯穿签名、沙箱及过滤防御面向

🛡️ 适配侦防、媒体、资安、政治等高敏感行业场景

## Deploy

```
go build ./cmd/server
mv server $GOPATH/bin/line
kill $(lsof -t -i :34282)
$GOPATH/bin/line -c=prod > $HOME/var/line/log 2>&1 &
```
