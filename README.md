# pgdriver-go
This is an open source driver for postgresql and written by pure golang.

Usage:

get connection:
```
pgUrl := "postgres://kong@127.0.0.1"
driver := &pgdriver_go.PGDriver{}
conn, err := driver.Open(pgUrl)
```

simple query:
```
sql := "select current_database(), current_schema(), current_user"
stmt, err := conn.Prepare(sql)
if err != nil {
    //...
}
rows, err := stmt.Query(nil)
//handl rows if err is nil...
```
prepare statement query:
```
sql := "select * from user where name=$1 and age=$2"
stmt, err := conn.Prepare(sql)
if err != nil {
    //...
}
rows, err := conn.Query([]driver.Value{"name", 25})
//handl rows if err is nil...
```
prepare statement (creat,update,delete):
```
sql := "update user set name=$1,age=$2 where id = $3"
stmt, err := conn.Prepare(sql)
if err != nil {
    //...
}
tx, err := conn.Begin()
result, err := stmt.Exec([]driver.Value{"name", 25, "id"})
if err != nil {
    tx.Rollback()
} else {
    tx.Commit()
}
```