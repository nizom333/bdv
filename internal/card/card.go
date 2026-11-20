// Пакет card — чистый рендер карточки задачи в текст и упорядоченный список ссылок.
package card

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/nizom333/bdv/internal/bd"
	"github.com/nizom333/bdv/internal/i18n"
)

// Input — всё, что нужно для рендера карточки.
type Input struct {
	Issue    *bd.Issue
	Detail   *bd.Detail   // nil, пока не загружено
	Comments []bd.Comment // nil, пока не загружено
	Children []*bd.Issue  // из tree.Forest.Children (все, отсортированы)
	Lookup   func(id string) (*bd.Issue, bool)
	Notice   string // предупреждение под шапкой; пусто — нет
	Blockers int    // открытые блокеры ждущей задачи (tree.Forest.Waiting/BlockedBy); > 0 — строка «⊘ waiting on N…»
	Width    int
	Style    string        // стиль glamour: "dark" | "light" | "notty" ("auto" опрашивает терминал)
	Text     *i18n.Catalog // тексты интерфейса; nil — английский
	// Карточка спеки (вкладка Specs): Progress — готовая полоса ProgressBar в чипах,
	// ChildrenTitle — название секции детей («Tickets»); "" — «Children».
	Progress      string
	ChildrenTitle string
}

const maxLinks = 9

// BlockedMark — знак заблокированной задачи (есть открытый блокер), красный.
const BlockedMark = "⊘"

// Палитра: только AdaptiveColor, светлый и тёмный фон.
var (
	// Accent — акцентный цвет: заголовки секций, номера ссылок, активная рамка.
	Accent = lipgloss.AdaptiveColor{Light: "62", Dark: "105"}
	// Muted — приглушённый цвет: неактивная рамка, линии.
	Muted = lipgloss.AdaptiveColor{Light: "250", Dark: "240"}

	faint    = lipgloss.NewStyle().Faint(true)
	bold     = lipgloss.NewStyle().Bold(true)
	accentSt = lipgloss.NewStyle().Foreground(Accent)
	ruleSt   = lipgloss.NewStyle().Foreground(Muted)
	warn     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "136", Dark: "214"})
	prioSt   = map[int]lipgloss.Style{
		0: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "160", Dark: "196"}),
		1: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "166", Dark: "208"}),
		3: faint,
		4: faint,
	}
	// labelColors — цвета чипов меток; цвет метки выбирается по хешу её текста.
	labelColors = []lipgloss.AdaptiveColor{
		{Light: "25", Dark: "75"},
		{Light: "90", Dark: "176"},
		{Light: "29", Dark: "79"},
		{Light: "130", Dark: "215"},
		{Light: "31", Dark: "117"},
		{Light: "125", Dark: "211"},
	}
)

// statusColors — цвета статусов; open и неизвестные — обычный цвет текста.
var statusColors = map[string]lipgloss.AdaptiveColor{
	"in_progress": {Light: "136", Dark: "220"},
	"blocked":     {Light: "160", Dark: "196"},
	"deferred":    {Light: "30", Dark: "44"},
	"closed":      {Light: "28", Dark: "77"},
}

// StatusColor — цвет статуса: in_progress жёлтый, blocked красный, deferred голубой,
// closed зелёный; open и неизвестный — пустой AdaptiveColor (обычный цвет).
func StatusColor(status string) lipgloss.AdaptiveColor { return statusColors[status] }

// StatusStyle — стиль иконки статуса: цвет статуса, у open без цвета.
func StatusStyle(status string) lipgloss.Style {
	if c, ok := statusColors[status]; ok {
		return lipgloss.NewStyle().Foreground(c)
	}
	return lipgloss.NewStyle()
}

// PrioStyle — стиль метки приоритета P<n>: P0 красный, P1 оранжевый, P3–P4 приглушённые.
func PrioStyle(p int) lipgloss.Style {
	if st, ok := prioSt[p]; ok {
		return st
	}
	return lipgloss.NewStyle()
}

// StatusBadge — бейдж статуса: текст из каталога t (nil — английский) на фоне цвета
// статуса; у open и неизвестного статуса фон нейтральный.
func StatusBadge(status string, t *i18n.Catalog) string {
	if t == nil {
		t = i18n.For(i18n.EN)
	}
	bg, fg := badgeColors(status)
	st := lipgloss.NewStyle().Bold(true).Padding(0, 1).Background(bg).Foreground(fg)
	return st.Render(CleanLine(t.Badge(status)))
}

// badgeColors — фон и текст бейджа статуса: фон — цвет статуса, у open и неизвестного
// нейтральный. Текст по контрасту WCAG: in_progress в светлой теме — чёрный 16 на 136
// (6.3:1; белый 231 давал 3.3:1); blocked — белый 231 на 160/196 (5.4:1 / 4.0:1).
func badgeColors(status string) (bg, fg lipgloss.AdaptiveColor) {
	c, ok := statusColors[status]
	switch {
	case !ok:
		return lipgloss.AdaptiveColor{Light: "252", Dark: "238"}, lipgloss.AdaptiveColor{Light: "235", Dark: "252"}
	case status == "blocked":
		return c, lipgloss.AdaptiveColor{Light: "231", Dark: "231"}
	case status == "in_progress":
		return c, lipgloss.AdaptiveColor{Light: "16", Dark: "16"}
	}
	return c, lipgloss.AdaptiveColor{Light: "231", Dark: "16"}
}

// Clean удаляет управляющие символы (ESC, C1 и т.п.), кроме \n и \t:
// данные задач приходят извне и не должны управлять терминалом.
func Clean(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		// Невалидные байты (например, сырой 0x9b) тоже отбрасываем.
		if r == utf8.RuneError && size == 1 {
			continue
		}
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// CleanLine — Clean для однострочных полей из bd: \n и \t становятся пробелом.
func CleanLine(s string) string { return lineReplacer.Replace(Clean(s)) }

var lineReplacer = strings.NewReplacer("\n", " ", "\t", " ")

// safeANSI оставляет в выводе glamour только SGR-последовательности
// (ESC [ параметры [0-9;:] m). Остальные ESC-последовательности (OSC до BEL
// или ST, прочие CSI, одиночный ESC со следующим байтом) и управляющие
// символы, кроме \n и \t, отбрасываются: glamour раскрывает HTML-сущности
// вроде &#27; уже после Clean.
func safeANSI(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}
		if r != 0x1b {
			if !unicode.IsControl(r) || r == '\n' || r == '\t' {
				sb.WriteString(s[i : i+size])
			}
			i += size
			continue
		}
		i++ // ESC
		if i >= len(s) {
			break
		}
		switch s[i] {
		case '[':
			j := i + 1
			for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == ';' || s[j] == ':') {
				j++
			}
			if j < len(s) && s[j] == 'm' {
				sb.WriteString(s[i-1 : j+1])
				i = j + 1
				break
			}
			// Прочий CSI: оставшиеся параметры, промежуточные байты и финальный.
			for j < len(s) && s[j] >= 0x20 && s[j] <= 0x3f {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
		case ']':
			j := i + 1
			for j < len(s) && s[j] != 0x07 && s[j] != 0x1b {
				j++
			}
			switch {
			case j >= len(s):
			case s[j] == 0x07:
				j++
			default: // ESC: ST (ESC \) съедаем целиком
				if j+1 < len(s) && s[j+1] == '\\' {
					j += 2
				}
			}
			i = j
		default:
			_, n := utf8.DecodeRuneInString(s[i:])
			i += n
		}
	}
	return sb.String()
}

// StatusIcon возвращает иконку статуса; неизвестный статус — «?».
func StatusIcon(status string) string {
	switch status {
	case "open":
		return "○"
	case "in_progress":
		return "◐"
	case "blocked":
		return "●"
	case "closed":
		return "✓"
	case "deferred":
		return "❄"
	}
	return "?"
}

func prio(p int) string { return PrioStyle(p).Render(fmt.Sprintf("P%d", p)) }

// Кеш рендереров glamour: без переноса — по стилю; для текста с таблицами — по стилю
// и ширине.
type rkey struct {
	style string
	width int
}

var (
	rmu       sync.Mutex
	renderers = map[rkey]*glamour.TermRenderer{}
)

// renderer — рендерер стиля style с переносом width; 0 — без переноса.
func renderer(style string, width int) (*glamour.TermRenderer, error) {
	rmu.Lock()
	defer rmu.Unlock()
	k := rkey{style, width}
	if r, ok := renderers[k]; ok {
		return r, nil
	}
	r, err := glamour.NewTermRenderer(glamour.WithStandardStyle(style), glamour.WithWordWrap(width))
	if err != nil {
		return nil, err
	}
	renderers[k] = r
	return r, nil
}

// tableSep — строка-разделитель шапки GFM-таблицы: «|---|:---|», «--- | :---:»,
// «| --- |». Без ведущей «|» ячеек должно быть хотя бы две, иначе это линия «---».
var tableSep = regexp.MustCompile(`^(\|\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?|:?-+:?\s*(\|\s*:?-+:?\s*)+\|?)$`)

// hasTable — есть ли в markdown таблица: строка-разделитель (tableSep) сразу под
// строкой с «|». Строки внутри блоков кода в ``` или ~~~ не считаются.
func hasTable(text string) bool {
	fence := "" // открывающий забор текущего блока кода; "" — вне блока
	prev := ""
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if f := fenceOf(l); f != "" {
			switch {
			case fence == "":
				fence = f
			// Закрывает забор того же символа не короче открывающего.
			case f[0] == fence[0] && len(f) >= len(fence):
				fence = ""
			}
			prev = ""
			continue
		}
		if fence != "" {
			continue
		}
		if strings.Contains(prev, "|") && tableSep.MatchString(l) {
			return true
		}
		prev = l
	}
	return false
}

// fenceOf — забор блока кода в начале строки («```», «~~~~» …) или "".
func fenceOf(l string) string {
	for _, c := range []string{"`", "~"} {
		n := len(l) - len(strings.TrimLeft(l, c))
		if n >= 3 {
			return l[:n]
		}
	}
	return ""
}

// markdown рендерит text через glamour по ширине width. Общий отступ (поле документа
// glamour) снимается, чтобы текст шёл вровень с заголовками карточки. safeANSI —
// последним, уже после переноса.
//
// Обычный текст glamour рендерит без переноса (WithWordWrap(0)): его перенос (reflow)
// не учитывал дефисы в ширине и ломал абзацы дважды. Переносим сами (wrapLine).
// Таблицу так не перенести — строки и разделители рассыпались бы, — поэтому текст
// с таблицей glamour рендерит по ширине сам (ячейки переносятся внутри), а строки
// только обрезаются на случай, если что-то всё же вышло шире.
func markdown(text, style string, width int) string {
	table := width > 0 && hasTable(text)
	rw := 0
	if table {
		rw = width
	}
	out := text
	if r, err := renderer(style, rw); err == nil {
		if s, err := r.Render(text); err == nil {
			out = s
		}
	}
	lines := strings.Split(out, "\n")
	indent := -1
	for i, l := range lines {
		vis := ansi.Strip(l)
		if strings.TrimSpace(vis) == "" {
			lines[i] = ""
			continue
		}
		if n := leading(vis); indent < 0 || n < indent {
			indent = n
		}
	}
	var res []string
	for _, l := range lines {
		if l == "" {
			res = append(res, "")
			continue
		}
		if indent > 0 {
			l = ansi.TruncateLeft(l, indent, "")
		}
		if table {
			res = append(res, ansi.Truncate(trimRightSpaces(l), width, "…"))
			continue
		}
		res = append(res, wrapLine(l, width)...)
	}
	// Пустые строки по краям и подряд не нужны.
	var outLines []string
	for _, l := range res {
		if l == "" && (len(outLines) == 0 || outLines[len(outLines)-1] == "") {
			continue
		}
		outLines = append(outLines, l)
	}
	for len(outLines) > 0 && outLines[len(outLines)-1] == "" {
		outLines = outLines[:len(outLines)-1]
	}
	return safeANSI(strings.Join(outLines, "\n"))
}

// leading — число ведущих пробелов строки.
func leading(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }

// listMarker — маркер пункта списка, как его выводит glamour: «• », «- », «* »,
// «1. », «[ ] », «[x] », «[✓] ».
var listMarker = regexp.MustCompile(`^(?:[•◦▪*-]|\d+\.|\[[ xX✓]\]) +`)

// wrapLine переносит строку по ширине width с висячим отступом: продолжение идёт
// под текстом — после ведущих пробелов и маркера пункта списка, если он есть.
// Слишком длинные слова и сплошные дефисы режутся жёстко. SGR, открытый на строке,
// переносится на продолжение, чтобы цвет и жирность не терялись на следующей строке.
func wrapLine(l string, width int) []string {
	if width <= 0 || ansi.StringWidth(l) <= width {
		return []string{l}
	}
	vis := ansi.Strip(l)
	n := leading(vis)
	m := ansi.StringWidth(listMarker.FindString(vis[n:]))
	if n+m > width/2 {
		m = 0
		if n > width/2 {
			n = 0
		}
	}
	body := l
	if n > 0 {
		body = ansi.TruncateLeft(body, n, "")
	}
	marker := ""
	if m > 0 {
		marker = ansi.Truncate(body, m, "")
		body = ansi.TruncateLeft(body, m, "")
	}
	parts := carrySGR(wrapParts(body, width-n-m))
	for i, p := range parts {
		if i == 0 {
			parts[i] = strings.Repeat(" ", n) + marker + p
		} else {
			parts[i] = strings.Repeat(" ", n+m) + p
		}
	}
	return parts
}

// wrapParts переносит s по словам на ширину lim; строки, всё ещё шире lim (сплошные
// дефисы, которые ansi.Wrap не ломает), режутся жёстко.
func wrapParts(s string, lim int) []string {
	lim = max(1, lim)
	var out []string
	for _, p := range strings.Split(ansi.Wrap(s, lim, ""), "\n") {
		p = trimRightSpaces(p)
		if ansi.StringWidth(p) <= lim {
			out = append(out, p)
			continue
		}
		for _, q := range strings.Split(ansi.Hardwrap(p, lim, true), "\n") {
			out = append(out, trimRightSpaces(q))
		}
	}
	return out
}

// trimRightSpaces убирает пробелы в конце строки, в том числе стоящие между
// CSI-последовательностями: ansi.Wrap оставляет пробел перед SGR в конце строки
// (например, у кода в строке с фоном), и строка выходит на колонку шире.
// Сами последовательности остаются, чтобы не сломать сбросы стилей.
func trimRightSpaces(s string) string {
	type tok struct {
		s     string
		esc   bool
		space bool
	}
	var toks []tok
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			if j < len(s) {
				j++
			}
			toks = append(toks, tok{s: s[i:j], esc: true})
			i = j
			continue
		}
		_, n := utf8.DecodeRuneInString(s[i:])
		toks = append(toks, tok{s: s[i : i+n], space: s[i] == ' '})
		i += n
	}
	last := -1
	for i, t := range toks {
		if !t.esc && !t.space {
			last = i
		}
	}
	var sb strings.Builder
	for i, t := range toks {
		if i <= last || t.esc {
			sb.WriteString(t.s)
		}
	}
	return sb.String()
}

// carrySGR повторяет в начале каждой строки SGR, действующие с конца предыдущей,
// и закрывает их сбросом в её конце: строки рисуются по отдельности (в рамке, в viewport).
func carrySGR(lines []string) []string {
	var active []string
	for i, l := range lines {
		prefix := strings.Join(active, "")
		active = sgrState(active, l)
		if prefix != "" {
			l = prefix + l
		}
		if len(active) > 0 {
			l += "\x1b[0m"
		}
		lines[i] = l
	}
	return lines
}

// sgrState — SGR, действующие после строки l, если до неё действовали active.
// Сброс (ESC[m, ESC[0m) очищает список.
func sgrState(active []string, l string) []string {
	for i := 0; i < len(l); i++ {
		if l[i] != 0x1b || i+1 >= len(l) || l[i+1] != '[' {
			continue
		}
		j := i + 2
		for j < len(l) && (l[j] >= '0' && l[j] <= '9' || l[j] == ';' || l[j] == ':') {
			j++
		}
		if j >= len(l) || l[j] != 'm' {
			continue
		}
		if p := l[i+2 : j]; p == "" || p == "0" {
			active = nil
		} else {
			active = append(active, l[i:j+1])
		}
		i = j
	}
	return active
}

// wrap переносит строку шапки по ширине панели только по пробелам: даты и ID
// не рвутся на дефисе; жёстко режутся лишь слова длиннее ширины. Стиль накладывается
// на каждую строку после переноса: многострочный Render lipgloss добил бы короткие
// строки пробелами.
func wrap(s string, width int, st *lipgloss.Style) string {
	lines := wrapWords(s, width)
	for i, l := range lines {
		if st != nil {
			l = st.Render(l)
		}
		lines[i] = l
	}
	return strings.Join(lines, "\n")
}

// wrapWords — перенос простого текста по пробелам на ширину width.
func wrapWords(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var lines []string
	cur := ""
	for _, w := range strings.Fields(s) {
		if ansi.StringWidth(w) > width {
			// Слово длиннее строки: целые куски — отдельными строками, хвост идёт дальше.
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			chunks := strings.Split(ansi.Hardwrap(w, width, false), "\n")
			lines = append(lines, chunks[:len(chunks)-1]...)
			w = chunks[len(chunks)-1]
		}
		switch {
		case cur == "":
			cur = w
		case ansi.StringWidth(cur)+1+ansi.StringWidth(w) <= width:
			cur += " " + w
		default:
			lines = append(lines, cur)
			cur = w
		}
	}
	return append(lines, cur)
}

// chips раскладывает готовые (уже стилизованные) чипы по строкам ширины width через пробел
// sep; чип шире строки обрезается.
func chips(items []string, sep string, width int) string {
	var lines []string
	cur, cw := "", 0
	for _, c := range items {
		w := ansi.StringWidth(c)
		if width > 0 && w > width {
			c, w = ansi.Truncate(c, width, "…"), width
		}
		switch {
		case cur == "":
			cur, cw = c, w
		case width > 0 && cw+ansi.StringWidth(sep)+w > width:
			lines = append(lines, cur)
			cur, cw = c, w
		default:
			cur += sep + c
			cw += ansi.StringWidth(sep) + w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return strings.Join(lines, "\n")
}

// labelChip — метка «#name» в цвете по хешу текста.
func labelChip(l string) string {
	h := fnv.New32a()
	h.Write([]byte(l))
	c := labelColors[h.Sum32()%uint32(len(labelColors))]
	return lipgloss.NewStyle().Foreground(c).Render("#" + l)
}

func date(t time.Time) string { return t.Format("2006-01-02") }

// pick — v, если оно не пустое, иначе fallback.
func pick(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

// builder накапливает текст и выдаёт номера ссылок.
type builder struct {
	sb    strings.Builder
	links []string
	width int
}

// line пишет строку, обрезая её по ширине панели.
func (b *builder) line(s string) {
	if b.width > 0 {
		s = ansi.Truncate(s, b.width, "…")
	}
	b.sb.WriteString(s + "\n")
}

// link печатает строку ссылки «[n] rest»; после девятой ссылки номер не выдаётся.
func (b *builder) link(id, rest string) {
	prefix := "    "
	if len(b.links) < maxLinks {
		b.links = append(b.links, id)
		prefix = accentSt.Render(fmt.Sprintf("[%d]", len(b.links))) + " "
	}
	b.line(prefix + rest)
}

// section — пустая строка, название акцентным цветом и тонкая линия до края панели.
func (b *builder) section(title string) {
	head := accentSt.Bold(true).Render(title)
	if rest := b.width - ansi.StringWidth(title) - 1; rest > 0 {
		head += " " + ruleSt.Render(strings.Repeat("─", rest))
	}
	b.sb.WriteString("\n")
	b.line(head)
}

// ref — «иконка P id  Заголовок»: иконка в цвете статуса, ID приглушённый.
func ref(status string, priority int, id, title string) string {
	return StatusStyle(status).Render(StatusIcon(status)) + " " + prio(priority) + " " +
		faint.Render(id) + "  " + title
}

func (b *builder) issueLine(is *bd.Issue) {
	id := CleanLine(is.ID)
	b.link(id, ref(is.Status, is.Priority, id, CleanLine(is.Title)))
}

func (b *builder) relatedLine(r bd.Related, withType bool) {
	id := CleanLine(r.ID)
	s := ref(r.Status, r.Priority, id, CleanLine(r.Title))
	if withType {
		s += " " + faint.Render("("+CleanLine(r.DependencyType)+")")
	}
	b.link(id, s)
}

// progressCells — ширина полосы прогресса в клетках.
const progressCells = 8

// ProgressBar — полоса прогресса спеки «███░░░░░ 3/8»: закрашенные клетки зелёным
// (цвет closed) по доле done/total с округлением, но полная — только когда закрыто всё;
// пустые приглушены. Без тикетов (total 0) — "".
func ProgressBar(done, total int) string {
	if total <= 0 {
		return ""
	}
	n := (done*progressCells*2 + total) / (total * 2)
	if done < total && n == progressCells {
		n--
	}
	n = max(0, min(n, progressCells))
	return StatusStyle("closed").Render(strings.Repeat("█", n)) +
		ruleSt.Render(strings.Repeat("░", progressCells-n)) + " " + fmt.Sprintf("%d/%d", done, total)
}

// Doc — карточка записи, которая не задача (память bd, формула, шаблон на вкладке
// Formulas): заголовок жирным с переносом, приглушённая строка meta (если есть) и текст
// как markdown. Текст bd очищается (Clean, CleanLine, safeANSI); строки не шире width.
func Doc(title, meta, text, style string, width int) string {
	b := &builder{width: width}
	b.sb.WriteString(wrap(CleanLine(title), width, &bold) + "\n")
	if m := CleanLine(meta); strings.TrimSpace(m) != "" {
		b.sb.WriteString(wrap(m, width, &faint) + "\n")
	}
	if strings.TrimSpace(text) != "" {
		b.sb.WriteString("\n" + markdown(Clean(text), style, width) + "\n")
	}
	lines := strings.Split(strings.TrimRight(b.sb.String(), "\n"), "\n")
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, width, "…")
	}
	return strings.Join(lines, "\n")
}

// TaskList — секция title (как в карточке: акцентное название и линия до края) со
// строками «[n] иконка id  заголовок» по порядку items; у ждущей задачи (waiting)
// после ID красный BlockedMark. Номера получают первые девять строк, links[i] — ID
// для клавиши i+1. Карточка группы приоритета. Ни одна строка не шире width.
func TaskList(title string, items []*bd.Issue, waiting func(id string) bool, width int) (string, []string) {
	b := &builder{width: width}
	b.section(title)
	for _, is := range items {
		id := CleanLine(is.ID)
		s := StatusStyle(is.Status).Render(StatusIcon(is.Status)) + " " + faint.Render(id)
		if waiting != nil && waiting(is.ID) {
			s += " " + StatusStyle("blocked").Render(BlockedMark)
		}
		b.link(is.ID, s+"  "+CleanLine(is.Title))
	}
	// Ведущая пустая строка секции — забота вызывающего.
	return strings.Trim(b.sb.String(), "\n"), b.links
}

// Render рендерит карточку; links[i] — ID для клавиши i+1, len ≤ 9.
// Полного ID в карточке нет: он в заголовке рамки панели. Ни одна строка не шире Width.
func Render(in Input) (string, []string) {
	is := in.Issue
	w := in.Width
	b := &builder{width: w}
	t := in.Text
	if t == nil {
		t = i18n.For(i18n.EN)
	}

	// Шапка: заголовок, чипы, метки, даты, пометка.
	b.sb.WriteString(wrap(CleanLine(is.Title), w, &bold) + "\n")
	meta := []string{StatusBadge(is.Status, t), prio(is.Priority)}
	if typ := CleanLine(is.IssueType); typ != "" {
		meta = append(meta, faint.Render("["+typ+"]"))
	}
	if is.Assignee != "" {
		meta = append(meta, accentSt.Render("@"+CleanLine(is.Assignee)))
	}
	if in.Progress != "" {
		meta = append(meta, in.Progress)
	}
	b.sb.WriteString(chips(meta, "  ", w) + "\n")
	if in.Blockers > 0 {
		// Перенос по простому тексту, затем знак в начале красится цветом blocked.
		line := wrap(BlockedMark+" "+t.WaitingOn(in.Blockers), w, nil)
		line = strings.Replace(line, BlockedMark, StatusStyle("blocked").Render(BlockedMark), 1)
		b.sb.WriteString(line + "\n")
	}
	if len(is.Labels) > 0 {
		tags := make([]string, len(is.Labels))
		for i, l := range is.Labels {
			tags[i] = labelChip(CleanLine(l))
		}
		b.sb.WriteString(chips(tags, " ", w) + "\n")
	}
	dates := t.CardCreated + " " + date(is.CreatedAt.Local()) + " · " + t.CardUpdated + " " + date(is.UpdatedAt.Local())
	if is.ClosedAt != nil {
		dates += " · " + t.CardClosed + " " + date(is.ClosedAt.Local())
		if is.CloseReason != "" {
			dates += " (" + CleanLine(is.CloseReason) + ")"
		}
	}
	b.sb.WriteString(wrap(dates, w, &faint) + "\n")
	if in.Notice != "" {
		b.sb.WriteString(wrap("⚠ "+CleanLine(in.Notice), w, &warn) + "\n")
	}

	// Текстовые секции.
	for _, s := range []struct{ title, text string }{{"Description", is.Description}, {"Notes", is.Notes}} {
		if strings.TrimSpace(s.text) != "" {
			b.section(s.title)
			b.sb.WriteString(markdown(Clean(s.text), in.Style, w) + "\n")
		}
	}
	// Поля спеки (v1.4): Spec — путь или ссылка одной строкой (длинная переносится
	// по ширине, а не обрезается), Design и Acceptance — markdown. Значения bd show
	// важнее значений из списка.
	spec, design, accept := is.SpecID, is.Design, is.AcceptanceCriteria
	if d := in.Detail; d != nil {
		spec, design, accept = pick(d.SpecID, spec), pick(d.Design, design), pick(d.AcceptanceCriteria, accept)
	}
	if s := strings.TrimSpace(CleanLine(spec)); s != "" {
		b.section("Spec")
		b.sb.WriteString(wrap(s, w, nil) + "\n")
	}
	for _, s := range []struct{ title, text string }{{"Design", design}, {"Acceptance", accept}} {
		if strings.TrimSpace(s.text) != "" {
			b.section(s.title)
			b.sb.WriteString(markdown(Clean(s.text), in.Style, w) + "\n")
		}
	}

	lookup := func(id string) (*bd.Issue, bool) {
		if in.Lookup == nil {
			return nil, false
		}
		return in.Lookup(id)
	}

	// Parent.
	if is.Parent != "" {
		b.section("Parent")
		if p, ok := lookup(is.Parent); ok {
			b.issueLine(p)
		} else {
			b.link(CleanLine(is.Parent), CleanLine(is.Parent))
		}
	}

	// Children (у спеки — Tickets).
	if len(in.Children) > 0 {
		title := in.ChildrenTitle
		if title == "" {
			title = "Children"
		}
		b.section(title)
		for _, c := range in.Children {
			b.issueLine(c)
		}
	}

	// Разбор связей: blocked by / blocks / related.
	var blockedBy, blocks, related []bd.Related
	if in.Detail != nil {
		for _, r := range in.Detail.Dependencies {
			switch r.DependencyType {
			case "blocks":
				blockedBy = append(blockedBy, r)
			case "parent-child":
			default:
				related = append(related, r)
			}
		}
		for _, r := range in.Detail.Dependents {
			switch r.DependencyType {
			case "blocks":
				blocks = append(blocks, r)
			case "parent-child":
			default:
				related = append(related, r)
			}
		}
	} else {
		for _, d := range is.Dependencies {
			if d.IssueID != is.ID || d.Type == "parent-child" {
				continue
			}
			r := bd.Related{ID: d.DependsOnID, DependencyType: d.Type}
			if t, ok := lookup(d.DependsOnID); ok {
				r.Title, r.Status, r.Priority = t.Title, t.Status, t.Priority
			}
			if d.Type == "blocks" {
				blockedBy = append(blockedBy, r)
			} else {
				related = append(related, r)
			}
		}
	}
	for _, g := range []struct {
		title    string
		items    []bd.Related
		withType bool
	}{{"Blocked by", blockedBy, false}, {"Blocks", blocks, false}, {"Related", related, true}} {
		if len(g.items) == 0 {
			continue
		}
		b.section(g.title)
		for _, r := range g.items {
			b.relatedLine(r, g.withType)
		}
	}

	// Comments: автор жирным, дата приглушённо, текст с чертой слева.
	if len(in.Comments) > 0 {
		b.section("Comments")
		bar := ruleSt.Render("▏")
		for i, c := range in.Comments {
			if i > 0 {
				b.sb.WriteString("\n")
			}
			b.line(bold.Render(CleanLine(c.Author)) + "  " + faint.Render(c.CreatedAt.Local().Format("2006-01-02 15:04")))
			for _, l := range strings.Split(markdown(Clean(c.Text), in.Style, w-2), "\n") {
				if l == "" {
					b.sb.WriteString(bar + "\n")
					continue
				}
				b.sb.WriteString(bar + " " + l + "\n")
			}
		}
	}

	return strings.TrimRight(b.sb.String(), "\n"), b.links
}

// Markdown — карточка задачи для буфера обмена (клавиша M): «# заголовок», строка
// «`id` · статус · P<n> · тип», затем секции «## Description» и «## Notes». Пустые секции
// и пустой тип пропускаются. Текст bd не очищается от разметки, только от управляющих
// символов (Clean): он уходит в буфер обмена, а оттуда в терминал или редактор.
func Markdown(is *bd.Issue) string {
	meta := "`" + CleanLine(is.ID) + "` · " + CleanLine(is.Status) + " · " + fmt.Sprintf("P%d", is.Priority)
	if typ := CleanLine(is.IssueType); typ != "" {
		meta += " · " + typ
	}
	parts := []string{"# " + CleanLine(is.Title), meta}
	for _, s := range []struct{ title, text string }{{"Description", is.Description}, {"Notes", is.Notes}} {
		if text := strings.TrimSpace(Clean(s.text)); text != "" {
			parts = append(parts, "## "+s.title, text)
		}
	}
	return strings.Join(parts, "\n\n")
}
