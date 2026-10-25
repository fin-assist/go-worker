# Проверки go-worker (FIN-252)

Unit: изолированный Clock с управляемыми каналами и наблюдаемым Stop. Проверяются all-or-nothing validation, прежний default5s, error/nil/panic restart, отсутствие restart без сигнала времени, отмена паузы, немедленная итерация Every, остановка ticker, отсутствие секретов в логах. Stdlib clock construction/Stop проверяется без ожидания времени.

Integration: реальные stdlib timers/tickers и запускаемые jobs. Независимые restart, ожидание удержанного cleanup после отмены, продолжение расписания после error. API: внешний пакет worker_test моделирует сервис с периодическим заданием и тремя независимо восстанавливаемыми consumers, проверяет cleanup и sanitized logs. Порядок задаётся каналами и ограниченными дедлайнами.

Каждый набор отдельно instrumentирует весь production модуль через одинаковый Go package inventory. Только точный internal/pgtest разрешён как исключение общего библиотечного шаблона; здесь такой пакет удалён вместе со starter SQL metadata, зависимости БД отсутствуют. Все Test/Fuzz/Example сохраняются в inventory. Тесты с unit/integration/API prefix имеют независимые80% native statements gates, combined не заменяет их. Guard требует реально прошедший Test без skip/fail и валидный непустой профиль. N/A только для фиксированной исходной identity go-library-template; go-worker его не наследует. Никакой собственной математики coverage нет.

make test/lint/cover/api запускаются с process timeout; compiler GOTOOLCHAIN=go1.26.0, native analyzer v2.20.1 с собственным auto toolchain. CI Python/regression, release/gorelease/migration/Renovate checks сохранены. Бизнес-задания сервисов не добавлены. Публичные callbacks должны соблюдать cancellation; отмена не может безопасно убить произвольную goroutine.

Локальная проверка compiler go1.26.0 darwin/amd64: unit100% (42/42), integration97.6% (41/42), API100% (42/42), combined100% (42/42), pinned native v2.20.1. make test/lint/cover/api и actionlint прошли с process timeout;134 Python regressions сохранены. CI на Linux проверяет текущий опубликованный head отдельно.
