# Go TCP Server

Example TCP server tối giản bằng Go standard library. Protocol theo từng dòng:

```text
PING       -> PONG
ECHO hello -> hello
QUIT       -> BYE, rồi đóng connection
```

## Chạy server

```bash
go run .
```

Server mặc định lắng nghe ở cổng `8080`.

## Thử bằng netcat

Mở terminal khác:

```bash
nc localhost 8080
```

Sau đó nhập từng dòng:

```text
PING
ECHO hello
QUIT
```

Kết quả:

```text
PONG
hello
BYE
```

Dừng server bằng `Ctrl+C`.

## TCP server lifecycle

```text
main
  --tạo context từ SIGINT/SIGTERM-->
serve
  --net.Listen("tcp", address)-->
listener
  --Accept() loop-->
connection
  --goroutine-->
handleConnection
  --read / process / write-->
close connection
```

- `net.Listen` tạo `listener`, nơi hệ điều hành gửi các TCP connection mới vào.
- `Accept()` chạy trong vòng lặp vì server cần nhận nhiều client, không chỉ một client.
- Mỗi connection chạy trong một goroutine để client chậm không chặn server nhận hay xử lý client khác.
- `handleConnection` có `defer conn.Close()`, nên mọi đường return đều đóng connection.
- Mỗi lần read và write đều đặt deadline. Client idle hoặc client không đọc response sẽ không giữ goroutine vô hạn.
- `signal.NotifyContext` nhận `Ctrl+C` (`SIGINT`) hoặc `SIGTERM`. Khi context bị cancel, server đóng listener, làm `Accept()` trả về và kết thúc accept loop. Connection đang xử lý tự kết thúc khi client xong hoặc deadline hết hạn.

## Kiểm tra

```bash
go test ./...
go test -race ./...
go vet ./...
```
