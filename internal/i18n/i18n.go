// Пакет i18n — каталог строк интерфейса bdv и выбор языка по окружению.
// Новый язык — один файл с переменной *Catalog и строкой в For.
package i18n

import (
	"fmt"
	"strconv"
	"strings"
)

// Lang — язык интерфейса; нулевое значение — английский.
type Lang int

const (
	EN Lang = iota
	RU
)

// String — код языка: "en" или "ru".
func (l Lang) String() string {
	if l == RU {
		return "ru"
	}
	return "en"
}

// Detect выбирает язык: BDV_LANG, затем LC_ALL, LC_MESSAGES, LANG — первая непустая.
// Значение, начинающееся с "ru", даёт русский, любое другое — английский.
func Detect(getenv func(string) string) Lang {
	for _, k := range []string{"BDV_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := getenv(k); v != "" {
			if strings.HasPrefix(strings.ToLower(v), "ru") {
				return RU
			}
			return EN
		}
	}
	return EN
}

// For — каталог языка l; неизвестный язык даёт английский.
// Каталог общий для всех вызовов: менять его нельзя.
func For(l Lang) *Catalog {
	if l == RU {
		return ru
	}
	return en
}

// HelpKey — строка справки: клавиша и действие.
type HelpKey struct{ Key, Action string }

// HelpSection — секция справки.
type HelpSection struct {
	Title string
	Keys  []HelpKey
}

// Catalog — все тексты интерфейса одного языка. Постоянные тексты — поля; тексты
// с подстановкой хранятся форматными строками (поля …Fmt) и выдаются методами.
// Каждое поле обязано быть заполнено (TestCatalogsComplete).
type Catalog struct {
	// Экраны загрузки и ошибки первой загрузки.
	Loading   string
	TooSmall  string // окно меньше 20x6: единственная строка экрана
	LoadError string
	ErrorHint string

	// Проект без базы beads: предложение bd init, подтверждение, запуск.
	NoDB             string // заголовок экрана вместо LoadError
	InitHint         string // подсказка клавиш экрана без базы
	InitConfirmFmt   string // вопрос подтверждения: абсолютный путь текущей папки
	InitPrefixFmt    string // префикс по умолчанию bd: имя текущей папки
	InitConfirmHint  string // клавиши подтверждения
	InitIDsHash      string // режим ID по умолчанию bd (хеш)
	InitIDsSeq       string // последовательные ID (c в подтверждении)
	InitIDsFailedFmt string // ошибка bd config set issue_id_mode counter после init
	Initializing     string // идёт bd init
	InitFailedFmt    string // ошибка bd init: код выхода или таймаут
	ExitStatusFmt    string // код выхода bd в тексте ошибки bd init

	// Дерево.
	TasksFmt     string // шапка дерева: число строк
	EmptyTree    string
	NoIssues     string
	NoMatchesFmt string // поиск без совпадений: запрос
	NoReady      string // пустое дерево из-за фильтра «готовые» (R)
	WaitingFmt   string // счётчик заблокированных задач в верхней строке; знак ⊘ — снаружи
	NotFoundFmt  string // задача не найдена: id

	// Статус-строка.
	UpdatedFmt       string // время последнего обновления
	RefreshFailedFmt string // первая строка ошибки обновления
	FilterFmt        string // закреплённый запрос
	ClosedToggle     string // показаны закрытые
	EventsToggle     string // показаны события
	NoGroupsToggle   string // группировка по приоритету выключена
	ReadyToggle      string // фильтр «готовые к работе» (R)
	CopiedFmt        string // y: скопирован ID
	CopiedTitle      string // Y: скопирован заголовок
	CopiedDesc       string // D: скопировано описание
	CopiedCard       string // M: скопирована карточка markdown
	NothingToCopy    string // копировать нечего: пустое поле или заголовок группы
	CopyFailedFmt    string
	MouseOffToggle   string // перехват мыши выключен (S)
	SearchHint       string // подсказка клавиши поиска справа
	HelpHint         string // подсказка клавиши справки справа
	CommentHint      string // подсказка клавиши комментария справа

	// Вкладки (v1.4): строка вкладок, заголовки левой панели, пустые панели и ошибки.
	TabTasks          string
	TabSpecs          string
	TabDecisions      string
	TabMemory         string
	TabFormulas       string
	SpecsFmt          string // заголовок панели Specs: число строк
	DecisionsFmt      string // заголовок панели Decisions: число строк
	MemoryFmt         string // заголовок панели Memory: число записей
	FormulasFmt       string // заголовок панели Formulas: число формул и шаблонов
	TabLoading        string // идёт чтение вкладки Memory или Formulas
	NoSpecs           string
	NoOpenSpecs       string // спеки есть, но все закрыты
	NoDecisions       string
	NoOpenDecisions   string // решения есть, но все закрыты
	NoMemories        string
	NoFormulas        string
	MemoryFailedFmt   string // ошибка bd memories: первая строка
	FormulasFailedFmt string // ошибка bd formula list или bd list --include-templates
	KindFormula       string // пометка строки формулы
	KindTemplate      string // пометка строки шаблона
	TabsHint          string // подсказка клавиш вкладок справа в статус-строке
	ReloadHint        string // подсказка r во вкладках Memory и Formulas

	// Поле комментария.
	CommentTitleFmt  string // заголовок рамки поля: id задачи
	CommentKeys      string // подсказка клавиш под полем
	CommentSending   string // идёт отправка
	CommentAdded     string // тост после успешной отправки
	CommentFailedFmt string // первая строка ошибки bd под полем
	// Таймаут отправки: текст таймаута и предупреждение, что bd мог успеть записать.
	CommentTimeoutFmt string

	// Ошибки bd.
	TimedOutFmt string // секунды строкой с DecimalSep
	DecimalSep  string

	// Карточка группы: заголовок «P1 · 3 tasks» и сводка по статусам.
	GroupTitleFmt string   // число и форма слова «задача»; P<n> — в заголовке рамки
	TaskForms     []string // формы слова «задача»: одна, несколько (2–4), много
	// plural выбирает индекс в TaskForms по числу; nil — английское правило.
	plural           func(n int) int
	StatusInProgress string
	StatusOpen       string
	StatusBlocked    string
	StatusDeferred   string
	StatusClosed     string

	// Бейджи статусов в карточке; неизвестный статус — в верхнем регистре как есть.
	BadgeOpen       string
	BadgeInProgress string
	BadgeBlocked    string
	BadgeDeferred   string
	BadgeClosed     string

	// Карточка.
	LinksFailedFmt    string
	CommentsFailedFmt string
	CardCreated       string
	CardUpdated       string
	CardClosed        string
	// Строка заблокированной задачи под чипами: число и форма «открытый блокер»; знак ⊘ — снаружи.
	WaitingOnFmt string
	BlockerForms []string

	// Экран настроек bd (v1.3, клавиша «,»).
	SettingsTitleFmt      string   // заголовок рамки: имя проекта
	SettingsTitle         string   // заголовок рамки без имени проекта
	SettingsLoading       string   // идёт чтение настроек
	SettingsLoadFailedFmt string   // ошибка чтения: первая строка ошибки bd
	SettingsLoadHint      string   // клавиши экрана ошибки чтения
	SettingsKeys          string   // клавиши экрана настроек
	SettingsSaving        string   // идёт bd config set/unset
	SettingsSections      []string // названия секций по порядку settings.Section
	SettingDefault        string   // пометка значения по умолчанию
	SettingFailedFmt      string   // ошибка записи под строкой: первая строка ошибки bd
	// Название, описание (под выбранной строкой) — по ключу bd; предупреждения — по Entry.Warn.
	SettingLabels map[string]string
	SettingDescs  map[string]string
	SettingWarns  map[string]string
	// Подтверждение изменения: «<key>: <старое> → <новое>».
	SettingConfirmFmt  string
	SettingConfirmKeys string
	InUseFmt           string   // удаление из списка: число, форма InUseForms, имя
	InUseForms         []string // «задача использует»: одна, несколько, много
	// Поле ввода числа.
	NumberPrompt  string
	NumberKeys    string
	NotInt        string
	NotNumber     string
	RangeFmt      string // от, до
	AtLeastFmt    string // от
	AboveOtherFmt string // ключ, его значение
	BelowOtherFmt string // ключ, его значение
	// Редактор списка.
	ListBuiltin   string
	ListCustom    string
	ListNone      string
	ListKeys      string
	ListAddPrompt string
	ListAddKeys   string
	BadName       string
	DuplicateFmt  string
	IsBuiltinFmt  string
	// Сброс сломал бы min_hash_length ≤ max_hash_length: ключ, значение, ключ, значение.
	UnsetBreaksFmt string
	// Упавшие bd config get: число, форма ReadFailedForms; r повторяет чтение.
	ReadFailedFmt   string
	ReadFailedForms []string
	// Подвал экрана: значения могут переопределяться окружением и глобальным конфигом.
	SettingsEnvNote string

	// Справка.
	HelpTitle  string
	HelpScroll string
	HelpLeft   []HelpSection
	HelpRight  []HelpSection
	HelpNote   string // строка во всю ширину под колонками: как выделять текст мышью

	// Командная строка: --help, аргументы, preflight.
	Usage             string
	HelpKeys          string // текст --help после строки использования
	ErrDBValue        string
	ErrUnknownFlagFmt string
	ErrExtraArgFmt    string
	ErrNoBD           string
	ErrNoBeadsFmt     string
}

// InitConfirm — вопрос перед bd init в папке dir.
func (c *Catalog) InitConfirm(dir string) string { return fmt.Sprintf(c.InitConfirmFmt, dir) }

// InitPrefix — префикс задач, который выберет bd init.
func (c *Catalog) InitPrefix(p string) string { return fmt.Sprintf(c.InitPrefixFmt, p) }

// InitIDsFailed — bd config set для последовательных ID завершился ошибкой msg.
func (c *Catalog) InitIDsFailed(msg string) string { return fmt.Sprintf(c.InitIDsFailedFmt, msg) }

// InitFailed — bd init завершился ошибкой msg.
func (c *Catalog) InitFailed(msg string) string { return fmt.Sprintf(c.InitFailedFmt, msg) }

// ExitStatus — код выхода bd code.
func (c *Catalog) ExitStatus(code int) string { return fmt.Sprintf(c.ExitStatusFmt, code) }

// Tasks — шапка дерева, n — число видимых строк.
func (c *Catalog) Tasks(n int) string { return fmt.Sprintf(c.TasksFmt, n) }

// SpecsTitle — заголовок панели Specs с n строками.
func (c *Catalog) SpecsTitle(n int) string { return fmt.Sprintf(c.SpecsFmt, n) }

// DecisionsTitle — заголовок панели Decisions с n строками.
func (c *Catalog) DecisionsTitle(n int) string { return fmt.Sprintf(c.DecisionsFmt, n) }

// MemoryTitle — заголовок панели Memory с n записями.
func (c *Catalog) MemoryTitle(n int) string { return fmt.Sprintf(c.MemoryFmt, n) }

// FormulasTitle — заголовок панели Formulas с n строками.
func (c *Catalog) FormulasTitle(n int) string { return fmt.Sprintf(c.FormulasFmt, n) }

// MemoryFailed — чтение памяти bd не удалось.
func (c *Catalog) MemoryFailed(msg string) string { return fmt.Sprintf(c.MemoryFailedFmt, msg) }

// FormulasFailed — чтение формул или шаблонов не удалось.
func (c *Catalog) FormulasFailed(msg string) string { return fmt.Sprintf(c.FormulasFailedFmt, msg) }

// NoMatches — пустое дерево при поиске q.
func (c *Catalog) NoMatches(q string) string { return fmt.Sprintf(c.NoMatchesFmt, q) }

// NotFound — задачи id нет в списке.
func (c *Catalog) NotFound(id string) string { return fmt.Sprintf(c.NotFoundFmt, id) }

// Updated — время t последнего успешного обновления.
func (c *Catalog) Updated(t string) string { return fmt.Sprintf(c.UpdatedFmt, t) }

// RefreshFailed — ошибка фонового обновления.
func (c *Catalog) RefreshFailed(msg string) string { return fmt.Sprintf(c.RefreshFailedFmt, msg) }

// Filter — закреплённый запрос поиска.
func (c *Catalog) Filter(q string) string { return fmt.Sprintf(c.FilterFmt, q) }

// Copied — подтверждение копирования id.
func (c *Catalog) Copied(id string) string { return fmt.Sprintf(c.CopiedFmt, id) }

// CopyFailed — ошибка копирования.
func (c *Catalog) CopyFailed(msg string) string { return fmt.Sprintf(c.CopyFailedFmt, msg) }

// CommentTitle — заголовок поля комментария к задаче id.
func (c *Catalog) CommentTitle(id string) string { return fmt.Sprintf(c.CommentTitleFmt, id) }

// CommentFailed — ошибка отправки комментария.
func (c *Catalog) CommentFailed(msg string) string { return fmt.Sprintf(c.CommentFailedFmt, msg) }

// CommentTimeout — таймаут отправки комментария msg: он мог сохраниться.
func (c *Catalog) CommentTimeout(msg string) string { return fmt.Sprintf(c.CommentTimeoutFmt, msg) }

// TimedOut — bd не ответил за sec секунд; дробная часть через DecimalSep.
func (c *Catalog) TimedOut(sec float64) string {
	s := strconv.FormatFloat(sec, 'f', -1, 64)
	return fmt.Sprintf(c.TimedOutFmt, strings.ReplaceAll(s, ".", c.DecimalSep))
}

// LinksFailed — ошибка bd show в карточке.
func (c *Catalog) LinksFailed(msg string) string { return fmt.Sprintf(c.LinksFailedFmt, msg) }

// CommentsFailed — ошибка bd comments в карточке.
func (c *Catalog) CommentsFailed(msg string) string { return fmt.Sprintf(c.CommentsFailedFmt, msg) }

// SettingsHeader — заголовок рамки экрана настроек проекта p ("" — без имени).
func (c *Catalog) SettingsHeader(p string) string {
	if p == "" {
		return c.SettingsTitle
	}
	return fmt.Sprintf(c.SettingsTitleFmt, p)
}

// SettingsLoadFailed — чтение настроек не удалось.
func (c *Catalog) SettingsLoadFailed(msg string) string {
	return fmt.Sprintf(c.SettingsLoadFailedFmt, msg)
}

// SettingFailed — запись настройки не удалась.
func (c *Catalog) SettingFailed(msg string) string { return fmt.Sprintf(c.SettingFailedFmt, msg) }

// SettingConfirm — строка подтверждения изменения ключа key со старого значения на новое.
func (c *Catalog) SettingConfirm(key, old, new string) string {
	return fmt.Sprintf(c.SettingConfirmFmt, key, old, new)
}

// InUse — n задач текущего списка используют статус или тип name.
func (c *Catalog) InUse(n int, name string) string {
	return fmt.Sprintf(c.InUseFmt, n, c.form(n, c.InUseForms), name)
}

// Range — число должно быть от a до b.
func (c *Catalog) Range(a, b string) string { return fmt.Sprintf(c.RangeFmt, a, b) }

// AtLeast — число должно быть не меньше a.
func (c *Catalog) AtLeast(a string) string { return fmt.Sprintf(c.AtLeastFmt, a) }

// AboveOther — число больше значения v ключа key.
func (c *Catalog) AboveOther(key, v string) string { return fmt.Sprintf(c.AboveOtherFmt, key, v) }

// BelowOther — число меньше значения v ключа key.
func (c *Catalog) BelowOther(key, v string) string { return fmt.Sprintf(c.BelowOtherFmt, key, v) }

// UnsetBreaks — сброс сделал бы ключ a (=av) больше ключа b (=bv).
func (c *Catalog) UnsetBreaks(a, av, b, bv string) string {
	return fmt.Sprintf(c.UnsetBreaksFmt, a, av, b, bv)
}

// ReadFailed — n настроек не прочитались.
func (c *Catalog) ReadFailed(n int) string {
	return fmt.Sprintf(c.ReadFailedFmt, n, c.form(n, c.ReadFailedForms))
}

// Duplicate — name уже есть в списке.
func (c *Catalog) Duplicate(name string) string { return fmt.Sprintf(c.DuplicateFmt, name) }

// IsBuiltin — name встроен в bd.
func (c *Catalog) IsBuiltin(name string) string { return fmt.Sprintf(c.IsBuiltinFmt, name) }

// form — форма слова из forms (одна, несколько, много) для числа n по правилу языка.
func (c *Catalog) form(n int, forms []string) string {
	i := 0
	if c.plural != nil {
		i = c.plural(n)
	} else if n != 1 {
		i = 2
	}
	return forms[i]
}

// GroupTitle — первая строка карточки группы с n задачами.
func (c *Catalog) GroupTitle(n int) string {
	return fmt.Sprintf(c.GroupTitleFmt, n, c.form(n, c.TaskForms))
}

// Waiting — счётчик n заблокированных задач в верхней строке.
func (c *Catalog) Waiting(n int) string { return fmt.Sprintf(c.WaitingFmt, n) }

// WaitingOn — строка карточки задачи, которую держат n открытых блокеров.
func (c *Catalog) WaitingOn(n int) string {
	return fmt.Sprintf(c.WaitingOnFmt, n, c.form(n, c.BlockerForms))
}

// StatusLabel — название статуса для сводки; неизвестный статус — как есть.
func (c *Catalog) StatusLabel(status string) string {
	switch status {
	case "in_progress":
		return c.StatusInProgress
	case "open":
		return c.StatusOpen
	case "blocked":
		return c.StatusBlocked
	case "deferred":
		return c.StatusDeferred
	case "closed":
		return c.StatusClosed
	}
	return status
}

// Badge — текст бейджа статуса; неизвестный статус — в верхнем регистре.
func (c *Catalog) Badge(status string) string {
	switch status {
	case "open":
		return c.BadgeOpen
	case "in_progress":
		return c.BadgeInProgress
	case "blocked":
		return c.BadgeBlocked
	case "deferred":
		return c.BadgeDeferred
	case "closed":
		return c.BadgeClosed
	}
	return strings.ToUpper(status)
}

// pluralRU — русское правило: 1 задача, 2–4 задачи, 5–20 задач, 21 задача…
func pluralRU(n int) int {
	n %= 100
	switch {
	case n >= 11 && n <= 14:
		return 2
	case n%10 == 1:
		return 0
	case n%10 >= 2 && n%10 <= 4:
		return 1
	}
	return 2
}

// ErrUnknownFlag — неизвестный флаг командной строки.
func (c *Catalog) ErrUnknownFlag(f string) string { return fmt.Sprintf(c.ErrUnknownFlagFmt, f) }

// ErrExtraArg — второй позиционный аргумент.
func (c *Catalog) ErrExtraArg(a string) string { return fmt.Sprintf(c.ErrExtraArgFmt, a) }

// ErrNoBeads — каталог .beads не найден от dir вверх.
func (c *Catalog) ErrNoBeads(dir string) string { return fmt.Sprintf(c.ErrNoBeadsFmt, dir) }
