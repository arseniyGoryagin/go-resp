## RESP protocol library in golang

Supports RESP 3.0

### Install

```
go get github.com/arseniyGoryagin/go-resp
```

```go
import resp "github.com/arseniyGoryagin/go-resp"
```

### Read

```go
r := resp.NewReader(conn)

v, err := r.Read()
if err != nil {
	log.Fatal(err)
}

if v.Typ == resp.STRING {
	fmt.Println(*v.StrValue) // OK
}
```

### Write

```go
w := resp.NewWriter(conn)

get, key := "GET", "mykey"
cmd := []resp.Value{
	{Typ: resp.BULK, StrValue: &get},
	{Typ: resp.BULK, StrValue: &key},
}

err := w.Write(resp.Value{Typ: resp.ARRAY, ArrValues: &cmd}) // *2\r\n$3\r\nGET\r\n$5\r\nmykey\r\n
```
