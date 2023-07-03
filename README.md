## tinkoff-api

[![Go Reference](https://pkg.go.dev/badge/github.com/jfk9w-go/tinkoff-api.svg)](https://pkg.go.dev/github.com/jfk9w-go/tinkoff-api)
[![Go Report](https://goreportcard.com/badge/github.com/jfk9w-go/tinkoff-api)](https://goreportcard.com/report/github.com/jfk9w-go/tinkoff-api)
[![Go Coverage](https://github.com/jfk9w-go/tinkoff-api/wiki/coverage.svg)](https://raw.githack.com/wiki/jfk9w-go/tinkoff-api/coverage.html)
[![CodeQL](https://github.com/jfk9w-go/tinkoff-api/workflows/CodeQL/badge.svg)](https://github.com/jfk9w-go/tinkoff-api/actions?query=workflow%3ACodeQL)

Клиент для веб-API Тинькофф-банка.

Возможности:
* авторизация
* получение информации о счетах, операциях и кассовых чеках
* получение информации о брокерских счетах и операциях

### Пример

Демонстрация авторизации и выполнения всего доступного функционала
с выводом данных в консоль. 

Код для авторизации будет запрошен из стандартного ввода.

Переменная `TINKOFF_SESSIONS_FILE` должна содержать путь к файлу (несуществующему) для
кэширования информации о сессии (и потенциального дальнейшего переиспользования).

```bash
TINKOFF_PHONE="+79999999999" TINKOFF_PASSWORD="123456" TINKOFF_SESSIONS_FILE="/tmp/tinkoff-sessions.json" go run example/main.go 
```
