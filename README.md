# Deploy

```
go build ./cmd/server
mv server $GOPATH/bin/line
kill $(lsof -t -i :34282)
$GOPATH/bin/line -c=prod > $HOME/var/line/log 2>&1 &
```
