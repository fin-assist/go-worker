# Изменения

Формат — [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/), версии и выпуск — [docs/releasing.md](docs/releasing.md). Каждый PR, который меняет API или поведение библиотеки, добавляет строку в «Unreleased»; при выпуске раздел получает номер версии и дату.

## [Unreleased]

## [v0.1.0] — 2026-10-25

### Добавлено

- FIN-252: go-worker выделяет Job/Every/Run с явными Logger/Clock/restart settings, немедленным первым scheduled запуском, error/nil/panic restart и ожиданием cancellation cleanup. Три независимых statements gate80, unit/integration/public API tests и consumer guide.

### Изменено

- FIN-254: подготовлен первый выпуск go-worker v0.1.0; накопленные изменения зафиксированы в версии. Публикация выполняется после слияния release PR и отдельного тега владельца.

- FIN-260: непривилегированный exact-true gate останавливает выключенный Renovate controller до Environment/secrets/checkout; CLI завершает работу до credentials/config/event/API. Явный bootstrap остаётся отдельным действием с обязательным credential, без активации. Runtime библиотеки и coverage не менялись.

- FIN-252: Every возвращает (Job,error) и валидирует конфигурацию; Run принимает Options и возвращает ctx.Err вместо void. Mutable RestartDelay заменён настройкой с прежним default5s. Error/panic payload/stack в журнале заменены безопасной reason classification. Starter migration metadata и PostgreSQL helper/dependencies удалены: библиотека не владеет БД.

- FIN-247: только исходный пустой go-library-template сообщает N/A для coverage после обязательного успешного выполнения всех трёх наборов. При появлении production statements автоматически действуют прежние total80; новые библиотеки не наследуют исключение. Проверки suites, required go, защиты и Controller Renovate сохранены; runtime и публичный API не менялись.
- FIN-243: независимые statements-профили unit/integration/API и обязательные пороги80 с готовым go-test-coverage v2.20.1; guards пустых/пропущенных наборов, исполнение Example/Fuzz и проверки публичного Go API. Только тестовый пакет internal/pgtest исключён из одинакового знаменателя всех трёх уровней; его тесты продолжают выполняться. Документирован эффект постоянно красного go на Renovate retry/close/quarantine и необходимость отдельного решения об операционной паузе. Runtime и экспортируемый API не менялись.
