# Изменения

Формат — [Keep a Changelog](https://keepachangelog.com/ru/1.1.0/), версии и выпуск — [docs/releasing.md](docs/releasing.md). Каждый PR библиотеки, включая документацию, тесты и CI, добавляет строку в «Unreleased»; исключение — полностью ботские PR Renovate только с зависимостями, их сводку по `go.mod` добавляет PR выпуска. При выпуске раздел получает номер версии и дату.

## [Unreleased]

- FIN-372: CI и конфигурации линтеров получают общий источник `fin-assist/go-ci` по неизменяемому SHA; локальные команды используют тот же профиль, обновление и откат выполняются одним PR. Runtime и публичный API не меняются.


## [v0.1.2] — 2026-10-26

### Исправлено

- FIN-264: проверка версии в CI сохраняет сравнение с предыдущим выпуском при публикации тега во время gorelease; чужие и изменившиеся теги отклоняются. Runtime и публичный API не меняются.

## [v0.1.1] — 2026-10-26

### Исправлено

- FIN-265: Every больше не запускает итерацию после отмены context, если во время медленной итерации уже пришёл tick (раньше select выбирал случайно). Добавлены unit- и integration-тесты этого случая (manualClock и SystemClock через Run) и unit-тесты точных полей журналов Every/Run. Публичный API не менялся, изменение совместимо.

### Изменено

- FIN-341: правила lint о процессном состоянии приведены к ядру go-library-template (FIN-322): секции `forbidigo` и `depguard` в `.golangci.yml` и `scripts/test_lint_policy.py` совпадают с шаблоном. Добавлены запреты `slog.With` и `os.Exit`; сняты правила вне ядра: `grpclog` и `grpclog/glogger`, `prometheus.Unregister`, `http.DefaultTransport`/`DefaultServeMux`/`PostForm`, `otel.Tracer`/`Meter`/`Handle` и `otel/log/global`, `os.UserHomeDir`/`UserConfigDir`/`UserCacheDir`, `os.TempDir`/`CreateTemp`/`MkdirTemp`, `(*os.File).Chdir`, `runtime.SetDefaultGOMAXPROCS`, `syscall`/`golang.org/x/sys`; принцип в целом проверяется на ревью. Runtime и экспортируемый API не менялись.
- FIN-338 (FIN-308, FIN-309): перенесены из go-library-template запреты `forbidigo`/`depguard` FIN-294 (глобальные логгеры, реестр метрик, `http`/`otel`, `os/signal`, окружение, `flag`, стандартные потоки, временный и рабочий каталоги, настройки runtime) с `scripts/test_lint_policy.py`, защита CI FIN-301 (`persist-credentials: false` в `lint`, `api`, `migrations`, `base_ref` через окружение) с `scripts/test_workflow_hardening.py`, правила `docs/releasing.md` и вступления CHANGELOG FIN-307 (строка в `[Unreleased]` от каждого PR, PR выпуска с ключом FIN и сводкой `go.mod`). Код библиотеки новым запретам уже соответствовал; runtime и экспортируемый API не менялись.
- FIN-316: из `AGENTS.md` (контракты FIN-252 и FIN-260) убрано временное правило запрашивать ревью только у Codex: квота Claude восстановлена, ревью снова запрашивается одним комментарием `@codex @claude` по общему правилу Workflow. Остальные условия контрактов сохранены; runtime и API библиотеки не менялись.

## [v0.1.0] — 2026-10-25

### Добавлено

- FIN-252: go-worker выделяет Job/Every/Run с явными Logger/Clock/restart settings, немедленным первым scheduled запуском, error/nil/panic restart и ожиданием cancellation cleanup. Три независимых statements gate80, unit/integration/public API tests и consumer guide.

### Изменено

- FIN-254: подготовлен первый выпуск go-worker v0.1.0; накопленные изменения зафиксированы в версии. Публикация выполняется после слияния release PR и отдельного тега владельца.

- FIN-260: непривилегированный exact-true gate останавливает выключенный Renovate controller до Environment/secrets/checkout; CLI завершает работу до credentials/config/event/API. Явный bootstrap остаётся отдельным действием с обязательным credential, без активации. Runtime библиотеки и coverage не менялись.

- FIN-252: Every возвращает (Job,error) и валидирует конфигурацию; Run принимает Options и возвращает ctx.Err вместо void. Mutable RestartDelay заменён настройкой с прежним default5s. Error/panic payload/stack в журнале заменены безопасной reason classification. Starter migration metadata и PostgreSQL helper/dependencies удалены: библиотека не владеет БД.

- FIN-247: только исходный пустой go-library-template сообщает N/A для coverage после обязательного успешного выполнения всех трёх наборов. При появлении production statements автоматически действуют прежние total80; новые библиотеки не наследуют исключение. Проверки suites, required go, защиты и Controller Renovate сохранены; runtime и публичный API не менялись.
- FIN-243: независимые statements-профили unit/integration/API и обязательные пороги80 с готовым go-test-coverage v2.20.1; guards пустых/пропущенных наборов, исполнение Example/Fuzz и проверки публичного Go API. Только тестовый пакет internal/pgtest исключён из одинакового знаменателя всех трёх уровней; его тесты продолжают выполняться. Документирован эффект постоянно красного go на Renovate retry/close/quarantine и необходимость отдельного решения об операционной паузе. Runtime и экспортируемый API не менялись.
