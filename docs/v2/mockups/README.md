# Макеты панели

Статические HTML-макеты новых страниц. Это не шаблоны Go и не код: разметка
повторяет классы `internal/panel/static/panel.css`, чтобы после реализации
страницы выглядели так же.

| Файл | Что показывает | Нормативное описание |
|---|---|---|
| [dashboard.html](dashboard.html) | `/admin`: статус, графики RAM/CPU/диска за 1 ч/6 ч/24 ч | `05-panel.md` §13 |
| [state.html](state.html) | `/admin/state`: состояния, их архивы модов, скачивание и восстановление | `05-panel.md` §10 |
| [config-form.html](config-form.html) | `/admin/config`: форма целиком, diff на клиенте, «Изменений нет» | `05-panel.md` §11 |

Открывать напрямую в браузере:

```bash
xdg-open docs/v2/mockups/dashboard.html
```

Стили подключаются относительным путём `../../../internal/panel/static/panel.css`,
поэтому макеты работают из checkout без сборки. Inline-стилей нет: только
классы `panel.css`, `button--*`, `card`, `card--warning`, `notice`, `badge`,
`tag`, `tag--danger`, `grid`, `tabs`, `config-row`, `actions`, `table-wrap`,
`log-panel`, `chart`, `chart-line`, `chart-line--info`, `chart-line--muted`,
`chart-grid`, `chart-label`, `metric-row`, `metric`, `metric-value`,
`metric-meta`.

Низкий диск показан в `dashboard.html` уведомлением; при `error` панель
блокирует мутации, но сервер продолжает работать. Графики — статический SVG
из нескольких точек: реальный шаблон строит `points` из метрик, `admin.js`
обновляет их раз в 15 секунд.
