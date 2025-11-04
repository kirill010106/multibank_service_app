# 🚀 Деплой VTB Multibank Backend на Timeweb Cloud

## 📋 Предварительные требования

1. **VPS на Timeweb Cloud** с установленными:
   - Ubuntu 20.04+ / Debian 11+
   - PostgreSQL 14+
   - Nginx (опционально, для reverse proxy)

2. **Доступ по SSH** с ключом (не пароль!)

3. **GitHub Secrets** настроены в репозитории

---

## 🔐 Шаг 1: Настройка GitHub Secrets

Зайди в **Settings** → **Secrets and variables** → **Actions** → **New repository secret**

Добавь следующие секреты:

```
TIMEWEB_HOST          - IP адрес сервера (например: 123.45.67.89)
TIMEWEB_USERNAME      - SSH пользователь (например: root или vtb)
TIMEWEB_SSH_KEY       - Приватный SSH ключ (содержимое ~/.ssh/id_rsa)
TIMEWEB_APP_DIR       - Директория приложения (например: /var/www/vtb-multibank)
TIMEWEB_DOMAIN        - Домен (например: api.vtbmultibank.ru) - для health check

# Также добавь секреты для .env файла на сервере (настроишь позже):
POSTGRES_URL          - postgresql://user:pass@localhost:5432/vtb_multibank
CLIENT_ID             - VTB API Client ID
CLIENT_SECRET         - VTB API Client Secret
JWT_SECRET            - Секрет для JWT токенов (сгенерируй случайную строку)
```

---

## 🖥️ Шаг 2: Подготовка сервера Timeweb Cloud

### 2.1. Подключись к серверу по SSH

```bash
ssh root@<your_timeweb_ip>
```

### 2.2. Установи зависимости

```bash
# Обновление системы
apt update && apt upgrade -y

# Установка PostgreSQL
apt install -y postgresql postgresql-contrib

# Установка Nginx (для reverse proxy)
apt install -y nginx

# Установка certbot (для SSL)
apt install -y certbot python3-certbot-nginx
```

### 2.3. Создай пользователя приложения

```bash
# Создание пользователя
useradd -m -s /bin/bash vtb
usermod -aG sudo vtb

# Создание директории приложения
mkdir -p /var/www/vtb-multibank
chown -R vtb:vtb /var/www/vtb-multibank
```

### 2.4. Настрой PostgreSQL

```bash
# Переключись на пользователя postgres
sudo -u postgres psql

-- В psql выполни:
CREATE DATABASE vtb_multibank;
CREATE USER vtb_user WITH ENCRYPTED PASSWORD 'your_secure_password';
GRANT ALL PRIVILEGES ON DATABASE vtb_multibank TO vtb_user;
\q
```

### 2.5. Создай .env файл

```bash
cd /var/www/vtb-multibank
nano .env
```

Содержимое `.env`:

```bash
# Config
CONFIG_PATH=config/prod.yaml

# Database
POSTGRES_URL=postgresql://vtb_user:your_secure_password@localhost:5432/vtb_multibank

# VTB API
CLIENT_ID=your_vtb_client_id
CLIENT_SECRET=your_vtb_client_secret

# JWT
JWT_SECRET=your_jwt_secret_generate_random_string

# Logging
LOG_LEVEL=2
LOG_FORMAT=json
```

Сохрани: `Ctrl+X` → `Y` → `Enter`

```bash
# Установи права доступа
chmod 600 .env
chown vtb:vtb .env
```

### 2.6. Настрой Nginx (опционально, но рекомендуется)

```bash
nano /etc/nginx/sites-available/vtb-multibank
```

Содержимое конфига:

```nginx
server {
    listen 80;
    server_name api.vtbmultibank.ru;  # Замени на свой домен

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Logs
    access_log /var/log/nginx/vtb-multibank-access.log;
    error_log /var/log/nginx/vtb-multibank-error.log;

    # Proxy to Go backend
    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
        
        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # Static Swagger UI (если нужно)
    location /swagger/ {
        proxy_pass http://localhost:8080/swagger/;
    }

    # Health check endpoint
    location /health {
        proxy_pass http://localhost:8080/health;
        access_log off;
    }
}
```

Сохрани и активируй конфиг:

```bash
ln -s /etc/nginx/sites-available/vtb-multibank /etc/nginx/sites-enabled/
nginx -t  # Проверка конфига
systemctl reload nginx
```

### 2.7. Настрой SSL с Let's Encrypt

```bash
certbot --nginx -d api.vtbmultibank.ru  # Замени на свой домен
```

### 2.8. Установи systemd service (рекомендуется)

```bash
# Скопируй файл service (будет загружен через GitHub Actions)
# Или создай вручную:
nano /etc/systemd/system/vtb-backend.service
```

Вставь содержимое из `backend/deploy/vtb-backend.service`, отредактируй пути:

```ini
WorkingDirectory=/var/www/vtb-multibank
ExecStart=/var/www/vtb-multibank/server
User=vtb
Group=vtb
```

Сохрани и активируй:

```bash
systemctl daemon-reload
systemctl enable vtb-backend
# Не запускай пока - сервис запустится после первого деплоя
```

---

## 🔑 Шаг 3: Настройка SSH ключа для GitHub Actions

### 3.1. Генерация SSH ключа (если нет)

На сервере Timeweb:

```bash
# От имени пользователя vtb
su - vtb
ssh-keygen -t ed25519 -C "github-actions-deploy"
# Сохрани в /home/vtb/.ssh/github_deploy
# БЕЗ пароля (просто Enter)

# Добавь публичный ключ в authorized_keys
cat /home/vtb/.ssh/github_deploy.pub >> /home/vtb/.ssh/authorized_keys
chmod 600 /home/vtb/.ssh/authorized_keys

# Скопируй ПРИВАТНЫЙ ключ для GitHub Secret
cat /home/vtb/.ssh/github_deploy
```

### 3.2. Добавь ключ в GitHub Secrets

Скопируй **полностью** содержимое приватного ключа (включая `-----BEGIN OPENSSH PRIVATE KEY-----` и `-----END OPENSSH PRIVATE KEY-----`) и добавь в **GitHub Secret** с именем `TIMEWEB_SSH_KEY`.

---

## 🚀 Шаг 4: Деплой через GitHub Actions

### 4.1. Первый деплой

```bash
# На локальной машине
git add .
git commit -m "feat: add Timeweb Cloud deployment config"
git push origin backend-pages
```

GitHub Actions автоматически:
1. ✅ Запустит тесты
2. ✅ Соберёт Go binary
3. ✅ Загрузит на сервер Timeweb
4. ✅ Запустит deployment скрипт
5. ✅ Проверит health check

### 4.2. Проверка деплоя

Открой **Actions** → **Deploy Backend to Timeweb** → посмотри логи

Если всё OK, проверь:

```bash
curl https://api.vtbmultibank.ru/health
# Ожидается: {"status":"ok"}

curl https://api.vtbmultibank.ru/swagger/index.html
# Должен открыться Swagger UI
```

---

## 🔄 Автоматические деплои

Теперь **каждый push в `backend-pages`** будет автоматически деплоить backend на Timeweb Cloud! 🎉

Процесс:
1. Пушишь код в `backend-pages`
2. GitHub Actions запускает тесты
3. Если тесты прошли → собирается binary
4. Binary загружается на сервер
5. Запускается `deploy.sh` → рестарт сервиса
6. Health check проверяет, что всё работает

---

## 🛠️ Troubleshooting

### Проблема: SSH connection failed

**Решение:**
```bash
# На сервере проверь SSH конфиг
nano /etc/ssh/sshd_config
# Убедись что:
PubkeyAuthentication yes
PasswordAuthentication no

systemctl restart sshd
```

### Проблема: Permission denied при деплое

**Решение:**
```bash
# На сервере
chown -R vtb:vtb /var/www/vtb-multibank
chmod +x /var/www/vtb-multibank/deploy.sh
```

### Проблема: Health check failed

**Решение:**
```bash
# На сервере проверь логи
tail -f /var/www/vtb-multibank/logs/app.log

# Или через systemd
journalctl -u vtb-backend -f

# Проверь переменные окружения
cat /var/www/vtb-multibank/.env

# Проверь подключение к базе
sudo -u postgres psql -c "\l" | grep vtb_multibank
```

### Проблема: Port 8080 already in use

**Решение:**
```bash
# Найди процесс
lsof -i :8080
# Или
netstat -tulpn | grep 8080

# Останови старый процесс
systemctl stop vtb-backend
# Или
pkill -f "server"
```

---

## 📊 Мониторинг

### Логи приложения

```bash
# Через systemd
journalctl -u vtb-backend -f

# Через файл
tail -f /var/www/vtb-multibank/logs/app.log
```

### Статус сервиса

```bash
systemctl status vtb-backend
```

### Nginx логи

```bash
tail -f /var/log/nginx/vtb-multibank-access.log
tail -f /var/log/nginx/vtb-multibank-error.log
```

---

## 🔄 Откат к предыдущей версии

Скрипт `deploy.sh` автоматически создаёт бэкапы в `/var/www/vtb-multibank-backups/`

Для отката:

```bash
cd /var/www/vtb-multibank
sudo systemctl stop vtb-backend

# Список бэкапов
ls -lh /var/www/vtb-multibank-backups/

# Откат на конкретную версию
cp /var/www/vtb-multibank-backups/server-20251104-150000 ./server
chmod +x ./server

sudo systemctl start vtb-backend
```

---

## ✅ Чеклист готовности к production

- [ ] PostgreSQL настроена и доступна
- [ ] `.env` файл создан с корректными секретами
- [ ] Nginx настроен с SSL (Let's Encrypt)
- [ ] SSH ключ добавлен в GitHub Secrets
- [ ] Systemd service настроен и активирован
- [ ] Firewall открыт на портах 80, 443 (UFW: `ufw allow 80,443/tcp`)
- [ ] Мониторинг настроен (логи, health checks)
- [ ] Backup стратегия для PostgreSQL (pg_dump cron job)
- [ ] Domain DNS указывает на IP сервера Timeweb

---

## 🎉 Готово!

Теперь твой backend автоматически деплоится на Timeweb Cloud при каждом пуше! 🚀
