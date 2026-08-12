# GSS Service

Dự án backend Go (Golang) được thiết kế theo cấu trúc **3 lớp tinh gọn (Handler ➡️ Service ➡️ Repository ➡️ Domain)**, đơn giản, dễ đọc và dễ mở rộng cho mọi lập trình viên.

---

## 🏗️ Cấu Trúc Dự Án (Project Structure)

```
gss/
├── cmd/
│   └── api/
│       └── main.go             # Entrypoint ứng dụng
├── configs/                    # Quản lý cấu hình (Viper) tại root (.env)
├── internal/
│   ├── app/                    # Khởi tạo & nạp các phụ thuộc (Dependency Injection)
│   ├── domain/                 # Struct dữ liệu cốt lõi (User, Timestamp) & Generated Mocks
│   ├── repository/             # Tương tác Cơ sở dữ liệu (GORM MySQL)
│   ├── service/                # Logic nghiệp vụ (Business Logic & Unit tests)
│   ├── handler/                # HTTP API (Gin Framework, DTOs, Router & Unit tests)
│   ├── errcode/                # Định nghĩa mã lỗi chuẩn
│   └── logger/                 # Structured Logger (slog)
├── .agents/                    # Agent Rules & Skills tự động
├── Makefile                    # Lệnh thao tác nhanh
└── README.md
```

---

## 🚀 Hướng Dẫn Nhanh

### 1. Cấu hình môi trường

```bash
cp .env.example .env
```

### 2. Khởi chạy MySQL local (Docker)

```bash
make docker-local-up
```

### 3. Chạy Server Backend

```bash
make run
```

- Server chạy tại: `http://localhost:8080`
- API Documentation (Swagger UI): `http://localhost:8080/swagger/index.html`

---

## 🛠️ Lệnh Makefile Thông Dụng

| Lệnh | Mô tả |
| :--- | :--- |
| `make run` | Chạy ứng dụng local |
| `make test` | Chạy toàn bộ kiểm thử Unit Tests (TDD) |
| `make generate` | Tự động sinh file Mock trong `internal/domain/mocks/` |
| `make build` | Biên dịch file chạy `bin/api` |
| `make docker-local-up` | Bật các service MySQL / MinIO |
| `make docker-local-down` | Tắt các service MySQL / MinIO |
