// Пакет app — Bubble Tea модель bdv: дерево задач слева, карточка справа, статус-строка внизу.
package app

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/nizom333/bdv/internal/bd"
	"github.com/nizom333/bdv/internal/card"
	"github.com/nizom333/bdv/internal/clip"
	"github.com/nizom333/bdv/internal/i18n"
	"github.com/nizom333/bdv/internal/tree"
)

// Source — источник данных; в проде это bd.Client.
type Source interface {
	List(ctx context.Context) ([]bd.Issue, error)
	Show(ctx context.Context, id string) (bd.Detail, error)
	Comments(ctx context.Context, id string) ([]bd.Comment, error)
	// AddComment — запись: bd comments add.
	AddComment(ctx context.Context, id, text string) error
	// Init — вторая запись: bd init в текущей папке; вывод stdout и stderr.
	Init(ctx context.Context) (string, error)
	// SetConfig — третья запись: bd config set key value (последовательные ID после init,
	// экран настроек).
	SetConfig(ctx context.Context, key, value string) error
	// ConfigUnset — четвёртая запись: bd config unset key (сброс настройки к умолчанию).
	ConfigUnset(ctx context.Context, key string) error
	// ConfigList — ключи базы: bd config list --json (чтение).
	ConfigList(ctx context.Context) (map[string]string, error)
	// ConfigGet — значение ключа и его location: bd config get key --json (чтение).
	ConfigGet(ctx context.Context, key string) (value, location string, err error)
	// Types — встроенные типы задач: bd types --json (чтение).
	Types(ctx context.Context) ([]bd.TypeInfo, error)
	// Memories — постоянная память bd: bd memories --json (чтение, вкладка Memory).
	Memories(ctx context.Context) ([]bd.Memory, error)
	// Formulas — формулы: bd formula list --json (чтение, вкладка Formulas).
	Formulas(ctx context.Context) ([]bd.Formula, error)
	// Templates — шаблоны: bd list --include-templates (чтение, вкладка Formulas).
	Templates(ctx context.Context) ([]bd.Issue, error)
}

// Options — зависимости и настройки модели; нулевые поля получают значения по умолчанию.
type Options struct {
	Source        Source
	InitialID     string
	PollEvery     time.Duration                              // 0 → 10s
	After         func(d time.Duration, msg tea.Msg) tea.Cmd // nil → tea.Tick
	Copy          func(string) error                         // nil → clip.Copy
	MarkdownStyle string                                     // "" → "dark"; "auto" не годится, см. main.markdownStyle
	Now           func() time.Time                           // nil → time.Now
	ListTimeout   time.Duration                              // 0 → 30s: предел на bd list
	DetailTimeout time.Duration                              // 0 → 15s: предел на bd show, bd comments, bd comments add, чтение настроек, вкладок Memory и Formulas и bd config set/unset
	Lang          i18n.Lang                                  // язык интерфейса; нулевое значение — английский
	Project       string                                     // имя проекта в верхней строке; "" — только сводка
	// InitAllowed — можно предлагать bd init: --db, BEADS_DIR и BEADS_DB не заданы (решает main).
	InitAllowed bool
	InitDir     string        // абсолютный путь текущей папки: где bd init создаст базу
	InitTimeout time.Duration // 0 → 120s: предел на bd init
	// StartNoDB — main не нашёл .beads: старт сразу на экране без базы, без первого bd list.
	StartNoDB bool
	// Term — куда писать сырые последовательности терминала, которых нет в командах
	// Bubble Tea v1 (alternate scroll, DECSET 1007); nil → os.Stdout.
	Term io.Writer
}

// Сообщения модели.
type (
	listMsg struct {
		issues []bd.Issue
		err    error
	}
	// gen — поколение кеша задачи на момент запроса; см. Model.gen.
	showMsg struct {
		id  string
		gen int
		d   bd.Detail
		err error
	}
	commentsMsg struct {
		id  string
		gen int
		c   []bd.Comment
		err error
	}
	// addCommentMsg — ответ bd comments add для задачи id.
	addCommentMsg struct {
		id  string
		err error
	}
	// initMsg — ответ bd init: вывод stdout и stderr вместе; cfgErr — ошибка
	// bd config set issue_id_mode counter после успешного init (если выбраны
	// последовательные ID).
	initMsg struct {
		out    string
		err    error
		cfgErr error
	}
	pollMsg       struct{}
	toastClearMsg struct{ seq int }
)

// toastFor — сколько держится тост копирования.
const toastFor = 2 * time.Second

const (
	focusTree   = "tree"
	focusDetail = "detail"
)

// rendered — кешированный рендер карточки.
type rendered struct {
	text  string
	links []string
}

// Model — состояние приложения.
type Model struct {
	o Options
	t *i18n.Catalog // тексты интерфейса языка o.Lang

	// Корневой контекст всех вызовов bd; отменяется при выходе, чтобы идущие
	// процессы bd убивались, а не оставались сиротами.
	ctx    context.Context
	cancel context.CancelFunc

	width, height int

	// Загрузка списка.
	inFlight bool
	// reloadAgain — после ответа идущего list запустить ещё один: он был выдан
	// до записи и может не видеть её (reloadFresh).
	reloadAgain bool
	loadErr     error // ошибка первой загрузки, пока данных нет
	lastOK      time.Time
	// Первая строка ошибки фонового обновления при имеющихся данных.
	// Живёт до следующего успешного списка; смена выбора её не снимает.
	refreshErr string

	// Подтверждение копирования; снимается toastClearMsg с тем же seq.
	toast    string
	toastSeq int

	// Дерево.
	forest   *tree.Forest
	prefix   string // общий префикс ID (tree.ShortPrefix), в строках дерева не показывается
	opts     tree.Options
	exp      tree.Expansion
	rows     []tree.Row
	cursor   int
	offset   int
	selected string // ID задачи или tree.GroupKey заголовка группы
	focus    string
	status   string // сообщение в статус-строке

	// Кеш подробностей по ID задачи.
	details         map[string]bd.Detail
	showNotice      map[string]string
	showPending     map[string]bool
	comments        map[string][]bd.Comment
	commentsNotice  map[string]string
	commentsPending map[string]bool
	renders         map[string]rendered
	// gen — поколение кеша задачи: растёт при каждой инвалидации. Ответ show/comments,
	// выданный в старом поколении, отбрасывается, чтобы данные до изменения не легли в кеш.
	gen map[string]int

	vp    viewport.Model
	links []string // ссылки текущей карточки: links[n-1] — цель клавиши n

	// Поиск: строка ввода в статус-строке; сам запрос живёт в opts.Query.
	input  textinput.Model
	typing bool // режим ввода: руны идут в запрос, а не в команды
	// Выбор на момент начала поиска; Esc возвращает его после поиска без совпадений.
	preSearch string

	// mouseOff — перехват мыши выключен клавишей S: терминал сам выделяет текст.
	mouseOff bool
	// altScrollOff — S выключил и alternate scroll (1007): вернуть при S или выходе.
	altScrollOff bool

	help    bool // экран справки поверх всего
	helpOff int  // прокрутка справки, если она не влезает

	// Поле комментария внизу экрана; пока оно открыто, клавиши идут в него.
	commenting bool
	commentID  string         // задача, к которой пишется комментарий
	ta         textarea.Model // текст комментария
	sending    bool           // идёт bd comments add: поле заблокировано
	commentErr string         // очищенная первая строка ошибки отправки
	// commentTimeout — ошибка отправки — таймаут: комментарий мог сохраниться.
	commentTimeout bool
	drafts         map[string]string
	// forceComments — задачи, чьи комментарии запрашиваются один раз, даже если
	// в списке comment_count ещё 0: только что добавлен комментарий.
	forceComments map[string]bool

	// bd init в проекте без базы (init.go).
	noDB        bool     // экран ошибки первой загрузки — вариант «нет базы» с клавишей i
	initConfirm bool     // подтверждение: только y запускает bd init
	initRunning bool     // идёт bd init: работает только Ctrl+C
	initFail    []string // очищенный вывод неудачного bd init
	initFailMsg string   // причина неудачи: код выхода или таймаут; "" — неудачи не было
	initDone    bool     // bd init успешен, идёт первая загрузка
	initOut     []string // последние строки вывода успешного bd init
	initSeq     bool     // в подтверждении выбраны последовательные ID (c); иначе хеш bd
	initCfgErr  string   // ошибка bd config set после успешного init: видна до первой загрузки и в статусе

	// Экран настроек bd (settings.go).
	set settingsState

	// Вкладки (tabs.go). Поля дерева выше (rows, cursor, offset, selected, opts.Query,
	// preSearch) принадлежат активной вкладке; состояние остальных лежит в views.
	tab   tab
	views [tabCount]tabView
	mem   lazyTab // вкладка Memory
	forms lazyTab // вкладка Formulas
}

// New создаёт модель; первая загрузка стартует в Init.
func New(o Options) Model {
	if o.PollEvery == 0 {
		o.PollEvery = 10 * time.Second
	}
	if o.After == nil {
		o.After = func(d time.Duration, msg tea.Msg) tea.Cmd {
			return tea.Tick(d, func(time.Time) tea.Msg { return msg })
		}
	}
	if o.Copy == nil {
		o.Copy = clip.Copy
	}
	if o.Term == nil {
		o.Term = os.Stdout
	}
	if o.MarkdownStyle == "" {
		// Запасной стиль без опроса терминала: "auto" спрашивал бы фон изнутри Update.
		o.MarkdownStyle = "dark"
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.ListTimeout == 0 {
		o.ListTimeout = 30 * time.Second
	}
	if o.DetailTimeout == 0 {
		o.DetailTimeout = 15 * time.Second
	}
	if o.InitTimeout == 0 {
		o.InitTimeout = 120 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	in := textinput.New()
	in.Prompt = "/"
	in.CharLimit = 200
	// Без мигания: курсор не порождает таймеров.
	in.Cursor.SetMode(cursor.CursorStatic)
	return Model{
		o:               o,
		t:               i18n.For(o.Lang),
		ctx:             ctx,
		cancel:          cancel,
		inFlight:        !o.StartNoDB, // Init сразу запускает List, кроме старта без базы
		noDB:            o.StartNoDB,
		opts:            tree.Options{GroupByPriority: true},
		exp:             tree.Expansion{},
		focus:           focusTree,
		details:         map[string]bd.Detail{},
		showNotice:      map[string]string{},
		showPending:     map[string]bool{},
		comments:        map[string][]bd.Comment{},
		commentsNotice:  map[string]string{},
		commentsPending: map[string]bool{},
		renders:         map[string]rendered{},
		gen:             map[string]int{},
		vp:              viewport.New(0, 0),
		input:           in,
		ta:              newTextarea(),
		drafts:          map[string]string{},
		forceComments:   map[string]bool{},
	}
}

// Init запускает первую загрузку и таймер опроса. При StartNoDB (main не нашёл .beads)
// первой загрузки нет: сразу экран без базы.
func (m Model) Init() tea.Cmd {
	poll := m.o.After(m.o.PollEvery, pollMsg{})
	if m.o.StartNoDB {
		return poll
	}
	return tea.Batch(m.listCmd(), poll)
}

// Update маршрутизирует сообщения.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.update(msg)
	return m, cmd
}

func (m *Model) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		m.settingsScroll()
		return nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case listMsg:
		return m.onList(msg)
	case showMsg:
		return m.onShow(msg)
	case commentsMsg:
		return m.onComments(msg)
	case addCommentMsg:
		return m.onAddComment(msg)
	case initMsg:
		return m.onInit(msg)
	case settingsMsg:
		return m.onSettings(msg)
	case settingSavedMsg:
		return m.onSettingSaved(msg)
	case memoriesMsg:
		return m.onMemories(msg)
	case formulasMsg:
		return m.onFormulas(msg)
	case pollMsg:
		// Опрос не накладывается на идущую загрузку, но таймер планируется всегда.
		// Без базы опроса нет: он только повторял бы ту же ошибку.
		next := m.o.After(m.o.PollEvery, pollMsg{})
		if m.noDBIdle() {
			return next
		}
		return tea.Batch(m.reload(), next)
	case toastClearMsg:
		// Устаревший таймер не снимает более новое подтверждение.
		if msg.seq == m.toastSeq {
			m.toast = ""
		}
		return nil
	}
	return nil
}

// --- Наблюдаемое состояние ---

// SelectedID — ID выбранной задачи, "#P<n>" для заголовка группы или "".
func (m Model) SelectedID() string { return m.selected }

// VisibleIDs — ID видимых строк левой панели активной вкладки по порядку; заголовки
// групп — "#P<n>", записи Memory — "memory:<ключ>", Formulas — "formula:<имя>"
// и "template:<ID>".
func (m Model) VisibleIDs() []string {
	ids := make([]string, len(m.rows))
	for i, r := range m.rows {
		ids[i] = r.Key()
	}
	return ids
}

// Focus — активная панель: "tree" или "detail".
func (m Model) Focus() string { return m.focus }

// AltScrollOff — S выключил alternate scroll терминала (DECSET 1007) и не включил обратно:
// main возвращает режим после выхода из Bubble Tea (AltScrollOn).
func (m Model) AltScrollOff() bool { return m.altScrollOff }

// --- Загрузка ---

func (m Model) listCmd() tea.Cmd {
	src, root, d, t := m.o.Source, m.ctx, m.o.ListTimeout, m.t
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(root, d)
		defer cancel()
		issues, err := src.List(ctx)
		return listMsg{issues: issues, err: timeoutErr(ctx, err, d, t)}
	}
}

// quit отменяет корневой контекст и завершает программу.
func (m *Model) quit() tea.Cmd {
	m.cancel()
	return tea.Quit
}

// Close отменяет корневой контекст: идущие вызовы bd убиваются. main вызывает его
// всегда после выхода из Bubble Tea — и тогда, когда программа завершилась по сигналу
// (SIGINT/SIGTERM) и quit не вызывался. Повторный вызов безопасен.
func (m Model) Close() { m.cancel() }

// timeoutError — bd не уложился в отведённое время; текст — из каталога t.
type timeoutError struct {
	after time.Duration
	t     *i18n.Catalog
}

func (e timeoutError) Error() string { return e.t.TimedOut(e.after.Seconds()) }

// timeoutErr заменяет ошибку вызова, оборванного таймаутом ctx, на timeoutError;
// прочие ошибки возвращает как есть.
func timeoutErr(ctx context.Context, err error, d time.Duration, t *i18n.Catalog) error {
	if err != nil && (errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded)) {
		return timeoutError{after: d, t: t}
	}
	return err
}

// reload запускает List, если предыдущий запрос не в полёте.
func (m *Model) reload() tea.Cmd {
	if m.inFlight {
		return nil
	}
	m.inFlight = true
	return m.listCmd()
}

// reloadFresh — reload после записи: если list уже в полёте, он выдан до записи,
// поэтому после его ответа запускается ещё один (один раз, сколько бы записей ни было).
func (m *Model) reloadFresh() tea.Cmd {
	if m.inFlight {
		m.reloadAgain = true
		return nil
	}
	return m.reload()
}

// onList применяет ответ list и при отложенном reloadFresh запускает следующий.
func (m *Model) onList(msg listMsg) tea.Cmd {
	m.inFlight = false
	cmd := m.applyList(msg)
	if m.reloadAgain {
		m.reloadAgain = false
		return tea.Batch(cmd, m.reload())
	}
	return cmd
}

func (m *Model) applyList(msg listMsg) tea.Cmd {
	if msg.err != nil {
		if m.forest == nil {
			m.loadErr = msg.err
			// «no beads database found» при разрешённом bd init — экран без базы.
			m.noDB = m.o.InitAllowed && isNoDB(msg.err)
			m.initDone, m.initOut = false, nil
		} else {
			// Данные остаются прежними; первая строка stderr идёт в статус-строку очищенной.
			m.refreshErr = card.CleanLine(firstLine(msg.err))
		}
		return nil
	}
	// Порядок важен: раскрытие замораживается по старому лесу и старым строкам,
	// пока они ещё не заменены новыми.
	old := m.forest
	if old != nil {
		rows := m.rows
		if m.tab != tabTasks {
			// Замораживаются строки дерева Tasks, а не строки активной вкладки.
			rows = tree.Rows(old, m.tasksOpts(), m.exp)
		}
		m.freezeExpansion(old, rows)
	}
	m.forest = tree.Build(msg.issues)
	// Префикс считается по всем задачам списка, а не только по видимым: иначе
	// он менялся бы от фильтров.
	m.prefix = tree.ShortPrefix(msg.issues)
	m.loadErr, m.refreshErr = nil, ""
	if m.initCfgErr != "" {
		// Ошибка bd config set: база создана, дерево открывается, предупреждение — в статусе.
		m.status = errorSt.Render("⚠ " + m.initCfgErr)
	}
	m.noDB, m.initDone, m.initOut, m.initFail, m.initFailMsg = false, false, nil, nil, ""
	m.initCfgErr = ""
	m.lastOK = m.o.Now()
	// Детей и заголовки связей карточка берёт из списка — старые рендеры неактуальны.
	clear(m.renders)

	if old == nil {
		m.selected = m.applyInitialID()
		old = m.forest
	} else {
		m.dropStale(old)
	}
	// Expansion, запрос и переключатели живут в m.exp и m.opts и не меняются.
	m.computeRows()
	// Замену исчезнувшей задаче ищем по старому лесу: в новом её может не быть.
	// force: рендеры сброшены, карточку перерисовываем, даже если выбор не сменился;
	// selectIndex же дозапросит подробности, выброшенные из кеша.
	return m.selectIndex(m.targetIndex(old), true)
}

// freezeExpansion закрепляет в m.exp видимые ветки (Row.HasChildren в строках rows
// старого леса old), которые раскрыты по умолчанию и не имеют ручного значения: обновление
// не сворачивает их под пользователем, даже если открытых потомков не осталось (story 44).
// Свёрнутые по умолчанию ветки, узлы-листья и новые узлы не замораживаются и следуют
// общим правилам, чтобы новые задачи появлялись в дереве (story 46). Row.Expanded
// не годится: при поиске предки раскрыты принудительно.
// Группы не замораживаются: без ручного значения группа раскрыта всегда
// (tree.Expansion.GroupExpanded), а ручное сворачивание и так лежит в m.exp.
func (m *Model) freezeExpansion(old *tree.Forest, rows []tree.Row) {
	for _, r := range rows {
		if r.IsGroup {
			continue
		}
		id := r.Issue.ID
		if _, ok := m.exp[id]; r.HasChildren && !ok && old.DefaultExpanded(id) {
			m.exp[id] = true
		}
	}
}

// dropStale инвалидирует кеш задач, у которых изменился updated_at или которые исчезли
// из списка: выбрасывает подробности, комментарии, пометки об ошибках и флаги идущих
// запросов и поднимает поколение, чтобы ответы на уже идущие запросы отбросились.
// Без флага ensureDetail выдаст новый запрос. Кеш неизменившейся задачи, в том числе
// пометка об ошибке, остаётся: запрос не повторяется.
func (m *Model) dropStale(old *tree.Forest) {
	ids := map[string]bool{}
	addKeys(ids, m.details)
	addKeys(ids, m.showNotice)
	addKeys(ids, m.showPending)
	addKeys(ids, m.comments)
	addKeys(ids, m.commentsNotice)
	addKeys(ids, m.commentsPending)
	for id := range ids {
		if n, ok := m.forest.Issue(id); ok {
			if o, ok := old.Issue(id); ok && o.UpdatedAt.Equal(n.UpdatedAt) {
				continue
			}
		}
		m.gen[id]++
		delete(m.details, id)
		delete(m.showNotice, id)
		delete(m.showPending, id)
		delete(m.comments, id)
		delete(m.commentsNotice, id)
		delete(m.commentsPending, id)
	}
}

func addKeys[V any](set map[string]bool, mp map[string]V) {
	for id := range mp {
		set[id] = true
	}
}

// applyInitialID делает InitialID видимой. Возвращает ID для выбора или "", если задачи нет.
func (m *Model) applyInitialID() string {
	id := m.o.InitialID
	if id == "" || !m.reveal(id) {
		return ""
	}
	return id
}

// reveal раскрывает предков id и его группу и включает фильтры, которые её скрывают,
// а фильтр «готовые» для ждущей задачи (tree.Forest.Waiting) выключает (Query не трогает).
// Если задачи нет в лесу, пишет об этом в статус и возвращает false.
func (m *Model) reveal(id string) bool {
	iss, ok := m.forest.Issue(id)
	if !ok {
		m.status = m.t.NotFound(card.CleanLine(id))
		return false
	}
	m.expandTo(id)
	if iss.Status == "closed" {
		m.opts.ShowClosed = true
	}
	if iss.IssueType == "event" {
		m.opts.ShowEvents = true
	}
	if m.forest.Waiting(id) {
		m.opts.ReadyOnly = false
	}
	return true
}

// expandTo раскрывает предков id и его группу; фильтры не трогает.
func (m *Model) expandTo(id string) {
	for _, a := range tree.Ancestors(m.forest, id) {
		m.exp[a] = true
	}
	m.exp[tree.GroupKey(tree.GroupOf(m.forest, id))] = true
}

// --- Дерево и выбор ---

// rebuild пересчитывает видимые строки и ставит курсор на выбранную задачу.
// Если она скрылась, выбор переходит на ближайшую видимую (tree.Nearest), иначе на первую строку.
func (m *Model) rebuild() tea.Cmd {
	if m.forest == nil {
		return nil
	}
	m.computeRows()
	return m.selectIndex(m.targetIndex(m.forest), false)
}

// computeRows пересчитывает строки активной вкладки без смены выбора; после него
// обязателен selectIndex.
func (m *Model) computeRows() { m.rows = m.tabRows(m.tab) }

// targetIndex — строка выбранной задачи или замены для исчезнувшей; без выбора — первая задача.
// Задача, скрытая в свёрнутой группе, заменяется заголовком группы. Иначе замена ищется
// через tree.Nearest по лесу f среди видимых строк: сначала в той же группе, затем везде,
// затем заголовок её группы, иначе первая задача. Исчезнувший заголовок — см. groupFallback.
func (m Model) targetIndex(f *tree.Forest) int {
	if m.tab != tabTasks {
		// Плоские вкладки: без групп и веток.
		return m.flatTarget()
	}
	if i := m.indexOf(m.selected); i >= 0 {
		return i
	}
	if m.selected == "" {
		return m.firstTask()
	}
	pos := make(map[string]int, len(m.rows))
	for i, r := range m.rows {
		pos[r.Key()] = i
	}
	if p, ok := tree.ParseGroupKey(m.selected); ok {
		return m.groupFallback(p)
	}
	visible := func(id string) bool {
		_, ok := pos[id]
		return ok
	}
	near := ""
	if m.opts.GroupByPriority {
		g := tree.GroupOf(f, m.selected)
		if i, ok := pos[tree.GroupKey(g)]; ok && !m.rows[i].Expanded {
			return i
		}
		near = tree.Nearest(f, m.selected, func(id string) bool {
			return visible(id) && tree.GroupOf(f, id) == g
		})
	}
	if near == "" {
		near = tree.Nearest(f, m.selected, visible)
	}
	if i, ok := pos[near]; ok {
		return i
	}
	if m.opts.GroupByPriority {
		if i, ok := pos[tree.GroupKey(tree.GroupOf(f, m.selected))]; ok {
			return i
		}
	}
	return m.firstTask()
}

// groupFallback — замена выбранному заголовку группы p, которого нет среди строк:
// первый корень приоритета p (группировку выключили), иначе следующий заголовок,
// иначе предыдущий, иначе первая задача.
func (m Model) groupFallback(p int) int {
	for i, r := range m.rows {
		if !r.IsGroup && r.Issue.Priority == p && m.forest.ParentID(r.Issue.ID) == "" {
			return i
		}
	}
	prev := -1
	for i, r := range m.rows {
		if !r.IsGroup {
			continue
		}
		if r.Group > p {
			return i
		}
		prev = i
	}
	if prev >= 0 {
		return prev
	}
	return m.firstTask()
}

// firstTask — индекс первой строки-задачи; если задач нет, 0.
func (m Model) firstTask() int {
	for i, r := range m.rows {
		if !r.IsGroup {
			return i
		}
	}
	return 0
}

func (m Model) indexOf(id string) int {
	if id == "" {
		return -1
	}
	for i, r := range m.rows {
		if r.Key() == id {
			return i
		}
	}
	return -1
}

// selectIndex выбирает строку i; при смене задачи карточка прокручивается в начало.
// force перерисовывает карточку, даже если ID не изменился.
func (m *Model) selectIndex(i int, force bool) tea.Cmd {
	if len(m.rows) == 0 {
		changed := m.selected != ""
		if changed {
			m.status = "" // как и ниже: сообщение живёт до смены выбора
		}
		m.selected, m.cursor, m.offset = "", 0, 0
		if changed || force {
			m.refreshCard()
		}
		return nil
	}
	i = max(0, min(i, len(m.rows)-1))
	id := m.rows[i].Key()
	changed := id != m.selected
	if changed && m.selected != "" {
		// Сообщение вроде «задача … не найдена» живёт до следующей смены выбора.
		m.status = ""
	}
	m.cursor, m.selected = i, id
	m.scrollTree()
	// Сводка группы зависит от фильтров и поиска, поэтому перерисовывается всегда; она дешёвая.
	if changed || force || m.rows[i].IsGroup {
		m.refreshCard()
	}
	if changed {
		m.vp.SetYOffset(0)
	}
	return m.ensureDetail()
}

// chromeRows — строки главного экрана вне тела панелей: верхняя строка, строка вкладок,
// две горизонтальные границы рамки и статус-строка.
const chromeRows = 5

// treeHeight — высота тела панелей: окно без chromeRows и открытого поля комментария.
// Если рядом с полем не остаётся minPanelBody строк, панели скрыты (0), а поле
// занимает всё тело.
func (m Model) treeHeight() int {
	h := max(0, m.height-chromeRows)
	if !m.commenting {
		return h
	}
	if h-commentBoxH >= minPanelBody {
		return h - commentBoxH
	}
	return 0
}

// scrollTree держит курсор в окне дерева.
func (m *Model) scrollTree() {
	h := m.treeHeight()
	if h <= 0 {
		m.offset = 0
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	m.offset = max(0, min(m.offset, len(m.rows)-h))
}

// firstLine — первая строка ошибки bd или текст прочей ошибки.
func firstLine(err error) string {
	var be *bd.Error
	if errors.As(err, &be) {
		return be.FirstLine()
	}
	return err.Error()
}
