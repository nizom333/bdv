package i18n

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

func TestDetect(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want Lang
	}{
		{"BDV_LANG=ru", map[string]string{"BDV_LANG": "ru"}, RU},
		{"BDV_LANG=en при LANG=ru_RU", map[string]string{"BDV_LANG": "en", "LANG": "ru_RU.UTF-8"}, EN},
		{"BDV_LANG=ru при LC_ALL=en_US", map[string]string{"BDV_LANG": "ru", "LC_ALL": "en_US.UTF-8"}, RU},
		{"LC_ALL=ru_RU.UTF-8", map[string]string{"LC_ALL": "ru_RU.UTF-8"}, RU},
		{"LC_ALL важнее LANG", map[string]string{"LC_ALL": "en_US.UTF-8", "LANG": "ru_RU.UTF-8"}, EN},
		{"LC_MESSAGES важнее LANG", map[string]string{"LC_MESSAGES": "ru_RU.UTF-8", "LANG": "en_US.UTF-8"}, RU},
		{"LC_ALL важнее LC_MESSAGES", map[string]string{"LC_ALL": "C", "LC_MESSAGES": "ru_RU.UTF-8"}, EN},
		{"LANG=en_US", map[string]string{"LANG": "en_US"}, EN},
		{"всё пусто", map[string]string{}, EN},
		{"LC_ALL пусто, LANG=ru", map[string]string{"LC_ALL": "", "LANG": "ru"}, RU},
		{"LANG=C", map[string]string{"LANG": "C"}, EN},
		{"LANG=C.UTF-8", map[string]string{"LANG": "C.UTF-8"}, EN},
		{"LC_ALL=C.UTF-8 при LANG=ru", map[string]string{"LC_ALL": "C.UTF-8", "LANG": "ru_RU.UTF-8"}, EN},
		{"LANG=POSIX", map[string]string{"LANG": "POSIX"}, EN},
		{"LC_MESSAGES=POSIX при LANG=ru", map[string]string{"LC_MESSAGES": "POSIX", "LANG": "ru_RU.UTF-8"}, EN},
		{"не ru — английский", map[string]string{"BDV_LANG": "de"}, EN},
	}
	for _, c := range cases {
		got := Detect(func(k string) string { return c.env[k] })
		if got != c.want {
			t.Errorf("%s: Detect = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestForUnknownIsEN(t *testing.T) {
	if For(Lang(42)) != For(EN) {
		t.Error("неизвестный язык должен давать английский каталог")
	}
	if For(RU) == For(EN) {
		t.Error("RU и EN — один каталог")
	}
}

// walkStrings вызывает f для каждой строки в v (поля структур, элементы срезов и массивов).
func walkStrings(path string, v reflect.Value, f func(path, s string)) {
	switch v.Kind() {
	case reflect.String:
		f(path, v.String())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			walkStrings(path+"."+v.Type().Field(i).Name, v.Field(i), f)
		}
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.Len() == 0 {
			f(path, "") // пустой срез — незаполненное поле
		}
		for i := 0; i < v.Len(); i++ {
			walkStrings(path+"["+strconv.Itoa(i)+"]", v.Index(i), f)
		}
	case reflect.Map:
		// Ключи по порядку: форма каталогов сравнивается как список путей.
		if v.Len() == 0 {
			f(path, "")
		}
		ks := v.MapKeys()
		sort.Slice(ks, func(i, j int) bool { return ks[i].String() < ks[j].String() })
		for _, k := range ks {
			walkStrings(path+"["+k.String()+"]", v.MapIndex(k), f)
		}
	case reflect.Pointer:
		if v.IsNil() {
			f(path, "")
			return
		}
		walkStrings(path, v.Elem(), f)
	}
}

// shape — структура каталога без текстов: длины срезов по путям.
func shape(v reflect.Value) []string {
	var out []string
	walkStrings("", v, func(p, _ string) { out = append(out, p) })
	return out
}

func TestCatalogsComplete(t *testing.T) {
	for _, l := range []Lang{EN, RU} {
		walkStrings(l.String(), reflect.ValueOf(For(l)), func(p, s string) {
			if strings.TrimSpace(s) == "" {
				t.Errorf("%s пусто", p)
			}
		})
	}
	// Справка одинаковой формы: те же секции и то же число клавиш.
	en, ru := shape(reflect.ValueOf(For(EN))), shape(reflect.ValueOf(For(RU)))
	if !reflect.DeepEqual(en, ru) {
		t.Errorf("каталоги разной формы:\nEN %v\nRU %v", en, ru)
	}
}

func TestEnglishCatalogLatin(t *testing.T) {
	walkStrings("EN", reflect.ValueOf(For(EN)), func(p, s string) {
		for _, r := range s {
			if unicode.Is(unicode.Cyrillic, r) {
				t.Errorf("%s содержит кириллицу: %q", p, s)
				return
			}
		}
	})
}

// Форматные строки принимают ровно свои аргументы: без %!… в выводе.
func TestCatalogFormats(t *testing.T) {
	for _, l := range []Lang{EN, RU} {
		c := For(l)
		for name, s := range map[string]string{
			"Tasks":          c.Tasks(6),
			"Updated":        c.Updated("15:04:05"),
			"RefreshFailed":  c.RefreshFailed("boom"),
			"NoMatches":      c.NoMatches("q"),
			"Filter":         c.Filter("q"),
			"Copied":         c.Copied("ep.1"),
			"CopyFailed":     c.CopyFailed("nope"),
			"NotFound":       c.NotFound("zz"),
			"TimedOut":       c.TimedOut(0.2),
			"LinksFailed":    c.LinksFailed("boom"),
			"CommentsFailed": c.CommentsFailed("boom"),
			"CommentTimeout": c.CommentTimeout("boom"),
			"ErrUnknownFlag": c.ErrUnknownFlag("--foo"),
			"ErrExtraArg":    c.ErrExtraArg("b"),
			"ErrNoBeads":     c.ErrNoBeads("/tmp"),
			"GroupTitle":     c.GroupTitle(3),
			"InitConfirm":    c.InitConfirm("/tmp/p"),
			"InitPrefix":     c.InitPrefix("p"),
			"InitFailed":     c.InitFailed("boom"),
			"InitIDsFailed":  c.InitIDsFailed("boom"),
			"Waiting":        c.Waiting(2),
			"WaitingOn":      c.WaitingOn(2),
			"ExitStatus":     c.ExitStatus(1),
			"SettingsHeader": c.SettingsHeader("p"),
			"SettingsLoad":   c.SettingsLoadFailed("boom"),
			"SettingFailed":  c.SettingFailed("boom"),
			"SettingConfirm": c.SettingConfirm("k", "a", "b"),
			"InUse":          c.InUse(2, "x"),
			"Range":          c.Range("1", "2"),
			"AtLeast":        c.AtLeast("1"),
			"AboveOther":     c.AboveOther("k", "1"),
			"BelowOther":     c.BelowOther("k", "1"),
			"Duplicate":      c.Duplicate("x"),
			"IsBuiltin":      c.IsBuiltin("x"),
			"UnsetBreaks":    c.UnsetBreaks("a", "1", "b", "2"),
			"ReadFailed":     c.ReadFailed(2),
		} {
			if strings.Contains(s, "%!") {
				t.Errorf("%v %s: %q", l, name, s)
			}
		}
	}
}

func TestTexts(t *testing.T) {
	en, ru := For(EN), For(RU)
	for _, c := range []struct{ got, want string }{
		{en.Tasks(6), "Tasks (6)"},
		{ru.Tasks(6), "Задачи (6)"},
		{en.Updated("15:04:05"), "updated 15:04:05"},
		{ru.Updated("15:04:05"), "обновлено 15:04:05"},
		{en.RefreshFailed("x"), "update failed: x"},
		{en.Copied("ep.1"), "copied ID: ep.1"},
		{ru.Copied("ep.1"), "скопирован ID: ep.1"},
		{en.NoMatches("q"), "Nothing found: q"},
		{en.EmptyTree, "No open tasks — press c to show closed"},
		{en.NoIssues, "No tasks"},
		{en.LoadError, "bd exited with an error"},
		{en.ErrorHint, "r — retry · q — quit"},
		{en.Loading, "Loading tasks…"},
		// Дробная часть: точка в английском, запятая в русском.
		{en.TimedOut(0.2), "bd did not respond in 0.2 s"},
		{en.TimedOut(30), "bd did not respond in 30 s"},
		{ru.TimedOut(0.2), "bd не ответил за 0,2 с"},
		// Таймаут отправки: комментарий мог сохраниться.
		{en.CommentTimeout("x"), "x; may have been saved — check comments before resending"},
		{ru.CommentTimeout("x"), "x; возможно, сохранён — проверьте комментарии перед повторной отправкой"},
		{en.GroupTitle(1), "1 task"},
		{en.GroupTitle(3), "3 tasks"},
		{en.GroupTitle(21), "21 tasks"},
		{ru.GroupTitle(1), "1 задача"},
		{ru.GroupTitle(3), "3 задачи"},
		{ru.GroupTitle(5), "5 задач"},
		{ru.GroupTitle(12), "12 задач"},
		{ru.GroupTitle(21), "21 задача"},
		{ru.GroupTitle(0), "0 задач"},
		{en.StatusLabel("in_progress"), "in progress"},
		{ru.StatusLabel("in_progress"), "в работе"},
		{en.StatusLabel("pinned"), "pinned"},
		{en.NoGroupsToggle, "no groups"},
		{ru.NoGroupsToggle, "без групп"},
		// v1.1: бейджи статусов, режимы словами, подсказки справа.
		{en.Badge("open"), "OPEN"},
		{en.Badge("in_progress"), "IN PROGRESS"},
		{en.Badge("blocked"), "BLOCKED"},
		{en.Badge("deferred"), "DEFERRED"},
		{en.Badge("closed"), "CLOSED"},
		{en.Badge("pinned"), "PINNED"},
		{ru.Badge("open"), "ОТКРЫТА"},
		{ru.Badge("in_progress"), "В РАБОТЕ"},
		{ru.Badge("blocked"), "БЛОК"},
		{ru.Badge("deferred"), "ОТЛОЖЕНА"},
		{ru.Badge("closed"), "ЗАКРЫТА"},
		{ru.Badge("pinned"), "PINNED"},
		{en.ClosedToggle, "closed"},
		{en.EventsToggle, "events"},
		{ru.ClosedToggle, "закрытые"},
		{ru.EventsToggle, "события"},
		{en.SearchHint, "/ search"},
		{en.TooSmall, "window too small"},
		{ru.TooSmall, "окно слишком маленькое"},
		{ru.SearchHint, "/ поиск"},
		// bd init в проекте без базы.
		{en.NoDB, "No beads database in this project."},
		{ru.NoDB, "В этом проекте нет базы beads."},
		{en.InitHint, "i — initialize (bd init) · r — retry · q — quit"},
		{ru.InitHint, "i — инициализировать (bd init) · r — повторить · q — выход"},
		{en.InitConfirm("/w/p"), "Initialize beads in /w/p?"},
		{ru.InitConfirm("/w/p"), "Инициализировать beads в /w/p?"},
		{en.InitPrefix("p"), "prefix: p"},
		{ru.InitPrefix("p"), "префикс: p"},
		{en.InitConfirmHint, "y — run bd init · c — switch IDs · n — cancel"},
		{ru.InitConfirmHint, "y — запустить bd init · c — сменить ID · n — отмена"},
		{en.InitIDsHash, "IDs: hash (bd default)"},
		{en.InitIDsSeq, "IDs: sequential 1, 2, 3"},
		{ru.InitIDsHash, "ID: хеш (по умолчанию bd)"},
		{ru.InitIDsSeq, "ID: последовательные 1, 2, 3"},
		{en.Waiting(3), "3 waiting"},
		{ru.Waiting(3), "3 ждут"},
		{en.WaitingOn(1), "waiting on 1 open blocker"},
		{en.WaitingOn(2), "waiting on 2 open blockers"},
		{ru.WaitingOn(1), "ждёт 1 открытого блокера"},
		{ru.WaitingOn(3), "ждёт 3 открытых блокеров"},
		{ru.WaitingOn(21), "ждёт 21 открытого блокера"},
		{en.Initializing, "initializing…"},
		{ru.Initializing, "инициализация…"},
		{en.InitFailed("x"), "bd init failed: x"},
		{ru.InitFailed("x"), "bd init не удался: x"},
		{en.ExitStatus(2), "exit status 2"},
		{ru.ExitStatus(2), "код выхода 2"},
		// v1.3: настройки.
		{en.SettingsHeader("demo-app"), "Settings · demo-app"},
		{ru.SettingsHeader("demo-app"), "Настройки · demo-app"},
		{en.SettingsHeader(""), "Settings"},
		{en.SettingsLoading, "loading settings…"},
		{en.SettingConfirm("issue_id_mode", "hash", "counter"), "issue_id_mode: hash → counter"},
		{en.InUse(1, "qa"), "1 task uses qa"},
		{en.InUse(3, "qa"), "3 tasks use qa"},
		{ru.InUse(1, "qa"), "1 задача использует qa"},
		{ru.InUse(3, "qa"), "3 задачи используют qa"},
		{ru.InUse(5, "qa"), "5 задач используют qa"},
		{en.SettingsSaving, "saving…"},
		{ru.SettingsSaving, "сохранение…"},
		{en.ReadFailed(1), "1 setting could not be read · r — retry"},
		{en.ReadFailed(2), "2 settings could not be read · r — retry"},
		{ru.ReadFailed(1), "не удалось прочитать 1 настройку · r — повторить"},
		{ru.ReadFailed(3), "не удалось прочитать 3 настройки · r — повторить"},
		{ru.ReadFailed(5), "не удалось прочитать 5 настроек · r — повторить"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}
