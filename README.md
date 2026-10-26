# go-worker

Библиотека lifecycle фоновых заданий «Понятных финансов» ([FIN-252](https://finassist.youtrack.cloud/issue/FIN-252), эпик FIN-248). Выделяет Job/Every/Run из internal/worker сервисного шаблона. Использует только стандартную библиотеку Go. Бизнес-задания, транзакции и распределённые блокировки остаются у сервисов; подключение сервисов выполняется отдельными задачами.

## Внедрение

```go
clock := worker.SystemClock{}
job, err := worker.Every("partition-check", time.Minute, logger, clock, checkPartitions)
if err != nil { return err }
err = worker.Run(ctx, worker.Options{
    Logger: logger,
    Clock: clock,
    RestartDelay: 5 * time.Second,
}, job, worker.Job{Name: "outbox", Run: dispatcher.Run})
if errors.Is(err, context.Canceled) { return nil }
return err
```

Импорт — `github.com/fin-assist/go-worker`, пакет worker. Версию выпускает владелец; теги в этом PR не создаются. Logger и Clock обязательны и должны быть concurrency-safe. SystemClock использует time.Timer/time.Ticker. RestartDelay=0 означает прежние пять секунд; отрицательное значение отклоняется.

Job.Run обязан учитывать context, завершать собственные goroutines и cleanup до возврата. Run запускает задания независимо, ждёт каждое после отмены и возвращает ctx.Err(); принудительно прервать игнорирующий отмену callback невозможно. Пустой список возвращается сразу с текущим ctx.Err(). До старта проверяется весь список: пустое имя или nil-функция не допускают частичного запуска. Контекст должен быть ненулевым, как в стандартных context API.

Ошибка, преждевременный nil и panic перезапускают только соответствующее задание после паузы. Отмена во время паузы освобождает timer и не запускает следующую попытку. Ошибка после отмены не вызывает restart. Каждый Run имеет собственные supervisors и ждёт их: повторный или одновременный Run допустим только если сами переданные jobs безопасны для такого использования.

Every создаёт Job без запуска goroutines. При вызове Job.Run ticker создаётся до первого немедленного вызова fn; это сохраняет исходную fixed-ticker семантику. Даже уже отменённый context не отменяет первый вызов при прямом Job.Run. Последующие вызовы последовательны, медленная итерация coalesces ticks по правилам time.Ticker; после отмены context следующая итерация не начинается, даже если tick уже ожидает (FIN-265); ошибка итерации не сдвигает расписание и не останавливает job. Panic обрабатывается supervisor Run. Каждый ticker останавливается при выходе, в том числе при panic. Интервал должен быть положительным; имя, fn, logger и clock обязательны.

Имена заданий — ограниченная конфигурация владельца процесса, а не пользовательские данные. Журнал содержит name, delay и фиксированную reason (error/returned/panic). Произвольные ошибки, panic payload, stack, токены и финансовые данные не логируются. Никаких глобальных logger/registry/restart settings нет. Clock.NewTimer/NewTicker должны быть concurrency-safe, возвращать ненулевые signals и поддерживать Stop после firing; callback времени не является hook бизнес-логики.

## Переход с internal/worker

- Job{Name, Run} сохраняет структуру; бизнес-код callback не переносится в библиотеку.
- Every теперь возвращает (Job, error), принимает явный Clock после logger и валидирует параметры до старта.
- Вместо Run(ctx, logger, jobs...) используется Run(ctx, Options, jobs...) с обработкой результата. Возврат ctx.Err() — явное изменение прежнего void API; context.Canceled при штатном завершении обычно переводится сервисом в nil.
- Mutable RestartDelay удалён. Прежний runtime default5s сохранён в Options; тесты получают собственный clock/delay без глобальных гонок.
- Наблюдаемое изменение logging: сообщения сохраняют факты restart/iteration failure, но произвольный error/panic/stack заменён безопасной классификацией reason. Поля job/delay задаёт конфигурация сервиса.

## Проверки

```sh
make test
make lint
make cover
make api
```

Compiler Go1.26 из go.mod; analyzer отдельно GOTOOLCHAIN=auto, pinned go-test-coverage/v2 v2.20.1. Каждый unit/integration/API уровень независимо требует80% statements; combined информационный. Все instrumented production пакеты одинаковы, исключить можно только точный internal/pgtest (в этой библиотеке его нет). Тесты запускают весь ./..., race/count1/timeout, inventory защищает Test/Fuzz/Example. Только исходный go-library-template имеет N/A; фиксированная identity в scripts/check-suite.sh не меняется. go-worker строго отклоняет пустой профиль.

Unit используют управляемые Clock/Timer/Ticker без БД/сети. Integration запускают реальные jobs/time и проверяют restart/cancel/join. API — внешний Go-потребитель с параллельными jobs, периодическим заданием и безопасными логами. Каналы и дедлайны задают порядок; sleeps не используются. Подробнее — [docs/testing.md](docs/testing.md).

CI сохраняет required go, lint, api (gorelease compatibility), migrations (PR), Python guards/release/CHANGELOG/Renovate tests и coverage artifacts. SQL и зависимостей БД нет. Release и Renovate-controller конфигурация наследуются; активация контроллера/защиты выполняется владельцем отдельно. До bootstrap own renovate-policy refs контроллер не включать. [Выпуск](docs/releasing.md), [политика Renovate](docs/renovate-policy.md).

FIN-260: только точный `RENOVATE_POLICY_ENABLED=true` допускает привилегированный controller job. Выключенный CLI не читает credentials/config/event и не делает API-запросов. Явный `--bootstrap` требует credential независимо от activation; own presets и protection подготавливает владелец до runtime merge, включение выполняет отдельной командой. См. [порядок настройки](docs/renovate-policy.md).

Проверка `api` устойчива к публикации тега во время CI: release-коммит сравнивается
с предыдущим выпуском, удалённый тег проверяется по commit до и после `gorelease`.
Подробности и границы исключения — [docs/releasing.md](docs/releasing.md#проверка-версии-в-ci-fin-264).
