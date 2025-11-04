# 🚀 Deployment Files

Конфигурация для автоматического деплоя VTB Multibank Backend на Timeweb Cloud через GitHub Actions.

## 📁 Структура

```
deploy/
├── deploy.sh              # Bash скрипт автоматического деплоя
├── vtb-backend.service    # Systemd service для автозапуска
├── TIMEWEB_SETUP.md       # Подробная инструкция по настройке
└── README.md              # Этот файл
```

## ⚡ Быстрый старт

### 1. Настрой GitHub Secrets

Добавь в **Settings** → **Secrets and variables** → **Actions**:

```
TIMEWEB_HOST          = 123.45.67.89
TIMEWEB_USERNAME      = vtb
TIMEWEB_SSH_KEY       = <содержимое приватного SSH ключа>
TIMEWEB_APP_DIR       = /var/www/vtb-multibank
TIMEWEB_DOMAIN        = api.vtbmultibank.ru
```

### 2. Подготовь сервер Timeweb

На сервере выполни:

```bash
# Создай директорию приложения
mkdir -p /var/www/vtb-multibank
cd /var/www/vtb-multibank

# Создай .env файл
cat > .env << 'EOF'
CONFIG_PATH=config/prod.yaml
POSTGRES_URL=postgresql://vtb_user:password@localhost:5432/vtb_multibank
CLIENT_ID=your_vtb_client_id
CLIENT_SECRET=your_vtb_client_secret
JWT_SECRET=your_random_jwt_secret
LOG_LEVEL=2
LOG_FORMAT=json
EOF

chmod 600 .env

# Настрой PostgreSQL (если ещё не настроен)
sudo apt install -y postgresql
sudo -u postgres createdb vtb_multibank
sudo -u postgres createuser vtb_user
sudo -u postgres psql -c "ALTER USER vtb_user WITH PASSWORD 'password';"
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE vtb_multibank TO vtb_user;"
```

### 3. Сделай push в backend-pages

```bash
git add .
git commit -m "feat: add Timeweb deployment config"
git push origin backend-pages
```

GitHub Actions автоматически задеплоит backend! ✅

---

## 📚 Подробная документация

Смотри **[TIMEWEB_SETUP.md](./TIMEWEB_SETUP.md)** для:
- Полной настройки сервера
- Установки Nginx + SSL
- Настройки systemd service
- Мониторинга и troubleshooting
- Стратегии отката версий

---

## 🔄 Workflow деплоя

1. **Push в `backend-pages`** → Триггер GitHub Actions
2. **Run Tests** → Все 77 тестов должны пройти
3. **Build Binary** → Сборка `server` executable
4. **Upload to Timeweb** → SCP файлов на сервер
5. **Run deploy.sh** → Рестарт сервиса с health check
6. **Health Check** → Проверка доступности API

---

## 🛠️ Локальное тестирование deploy.sh

Можно протестировать скрипт локально на сервере:

```bash
cd /var/www/vtb-multibank
sudo bash deploy.sh
```

Скрипт выполнит:
- ✅ Backup текущей версии
- ✅ Остановку старого процесса
- ✅ Запуск нового процесса
- ✅ Health check на `localhost:8080/health`
- ✅ Автоматический rollback при ошибке

---

## 📊 Мониторинг

### Проверка статуса сервиса

```bash
# Через systemd
systemctl status vtb-backend

# Или напрямую
curl http://localhost:8080/health
```

### Просмотр логов

```bash
# Приложение
tail -f /var/www/vtb-multibank/logs/app.log

# Systemd
journalctl -u vtb-backend -f
```

---

## 🔐 Безопасность

**Важно!** Проверь:
- [ ] SSH доступ только по ключу (отключи пароль)
- [ ] `.env` файл с правами `600`
- [ ] Firewall открыт только на 80, 443 (UFW/iptables)
- [ ] PostgreSQL слушает только localhost
- [ ] JWT_SECRET — случайная строка минимум 32 символа

---

## 🎉 Готово!

Теперь каждый push в `backend-pages` автоматически деплоит backend на Timeweb Cloud! 🚀

**Swagger UI**: https://api.vtbmultibank.ru/swagger/index.html  
**Health Check**: https://api.vtbmultibank.ru/health
