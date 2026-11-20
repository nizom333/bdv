[English](README.md) · [Русский](README.ru.md) · **Oʻzbekcha**

# bdv

**bdv — [beads](https://github.com/steveyegge/beads) (`bd`) vazifa trekeri uchun tez va minimalistik terminal interfeysi (TUI): siz va dasturlash uchun sunʼiy intellekt agentlaringiz (Claude Code, Codex, Cursor va boshqalar) beads’da yuritadigan vazifalar daraxtini qoʻshni terminalda real vaqtda kuzating.**

[![CI](https://github.com/nizom333/bdv/actions/workflows/ci.yml/badge.svg)](https://github.com/nizom333/bdv/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/nizom333/bdv)](https://github.com/nizom333/bdv/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/nizom333/bdv)](go.mod)
[![License: MIT](https://img.shields.io/github/license/nizom333/bdv)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/nizom333/bdv)](https://goreportcard.com/report/github.com/nizom333/bdv)

```text
demo-app  ◐ 2 in progress · ○ 8 open · ● 0 blocked · ⊘ 2 waiting
 Tasks │ Specs 2 │ Decisions 0 │ Memory │ Formulas
╭─ ◆ Tasks (10) ────────────────────────╮╭─ demo-app-k3f ──────────────────────────────────────────────╮
│ ▾ P0 · 1                              ││ Checkout flow                                               │
│     ○ x2b Prices rounded wrong for JPY││  IN PROGRESS   P1  [epic]                                   │
│ ▾ P1 · 7                              ││ #shop #payments                                             │
│   ▾ ◐ k3f Checkout flow               ││ created 2026-11-18 · updated 2026-11-20                     │
│       ◐ k3f.2 Payment provider integr…││                                                             │
│       ○ P2 k3f.3 ⊘ Order confirmation…││ Description ─────────────────────────────────────────────── │
│       ○ P2 k3f.4 ⊘ Retry failed payme…││ Customers can pay for the cart in one step.                 │
│   ▾ ○ p7a Product search              ││                                                             │
│       ○ p7a.1 Search index for produc…││ ## Scope                                                    │
│       ○ P2 p7a.2 Typo-tolerant queries││                                                             │
│ ▾ P3 · 1                              ││ • Card payments through the payment provider                │
│     ○ m4e Upgrade Go toolchain        ││ • Order confirmation email                                  │
│ ▾ P4 · 1                              ││ • Retry on network errors                                   │
│     ○ q8r Dark mode for admin panel   ││                                                             │
│                                       ││ ## Out of scope                                             │
│                                       ││                                                             │
│                                       ││ Gift cards and split payments.                              │
│                                       ││                                                             │
│                                       ││ Children ────────────────────────────────────────────────── │
╰───────────────────────────────────────╯╰─────────────────────────────────────────────────────────────╯
updated 02:11:12                                                [ ] tabs · / search · m comment · ? help
```

Yuqori qatorda loyiha nomi va statuslar boʻyicha xulosa, uning ostida — varaqlar (tabs). Pastda chapda ota–bola bogʻlanishlari asosidagi vazifalar daraxti, oʻngda tanlangan vazifaning toʻliq kartochkasi, eng pastda holat qatori. Maʼlumotlar har 10 soniyada oʻzi yangilanadi, shuning uchun agent vazifalarni yaratib, oʻzgartirib va yopib turganda bdv’ni ochiq qoldirish mumkin. bdv interfeysi ingliz va rus tillarida; oʻzbek tilidagi interfeys hozircha yoʻq.

bdv maʼlumotlaringizga ehtiyotkorlik bilan munosabatda boʻladi. Yozish amallari faqat bir nechta: izoh qoʻshish (`m`), u `bd comments add <id> -f <vaqtinchalik fayl>` sifatida yuboriladi; bazasi yoʻq loyihada `bd init`; agar ketma-ket ID tanlangan boʻlsa, oʻsha `bd init` dan keyin darhol `bd config set issue_id_mode counter`; sozlamalar ekranidan (`,`) `bd config set` / `bd config unset` — har biri `y` bilan tasdiqlangandan keyin. `bd` ning qolgan barcha chaqiruvlari `--readonly` bilan ishlaydi.

## Imkoniyatlar

- **Vazifalar daraxti** ota–bola bogʻlanishlari boʻyicha: ochiqlari yuqorida, yopilganlar va hodisalar soʻramaguningizcha yashirin (`c`, `e`), ID loyihaning umumiy prefiksisiz koʻrsatiladi.
- **Ustuvorlik guruhlari**: ildiz vazifalar `P0`…`P4` sarlavhalari ostida (`p` yoqadi va oʻchiradi); guruh kartochkasi uning vazifalarini sanab beradi.
- **Varaqlar**: Tasks, Specs (`███░░░░░ 3/8` progressli epiklar), Decisions, Memory (`bd memories`) va Formulas (`bd formula list` va shablonlar).
- **Vazifa kartochkasi**: markdown tavsif, eslatmalar, spetsifikatsiya maydonlari (Spec, Design, Acceptance), ota, bolalar, blokerlar, bogʻliq vazifalar va izohlar; `1`–`9` havolalar boʻyicha oʻtadi.
- **Qidiruv** yozish jarayonida darhol filtrlaydi, kirill yozuvi ham.
- **Bloklangan vazifalar** qizil `⊘` bilan belgilanadi va yuqori qatorda sanaladi; `R` faqat ishga tayyorlarini qoldiradi.
- **Izohlar**: `m` — yozish, `Ctrl+S` — yuborish.
- **bd sozlamalari ekrani** (`,`): loyihaning `bd config` qiymatlarini oʻqish va har bir oʻzgarishni tasdiqlab oʻzgartirish.
- Bazasiz papkada **`bd init`** taklifi, xesh yoki ketma-ket ID bilan.
- ID, sarlavha, tavsif yoki butun kartochkani markdown koʻrinishida **nusxalash** (`y`, `Y`, `D`, `M`).
- **Tillar**: ingliz va rus interfeysi (`BDV_LANG`).
- Sichqoncha gʻildiragi, kichik oynalar uchun tor rejim, terminalning yorugʻ va qorongʻi mavzulari.

## Oʻrnatish

bdv hamma narsani `bd` CLI orqali oʻqiydi, shuning uchun avval [beads](https://github.com/steveyegge/beads) ni oʻrnating va `bd` `PATH` da ekanini tekshiring.

### Homebrew (macOS, Linux)

```sh
brew install nizom333/tap/bdv
```

Tap’da bdv macOS va Linux (amd64, arm64) uchun tayyor binar fayllar bilan Homebrew cask sifatida joylashgan. Yangilash: `brew upgrade bdv`.

### Scoop (Windows)

```powershell
scoop bucket add nizom333 https://github.com/nizom333/homebrew-tap
scoop install nizom333/bdv
```

### Debian, Ubuntu, Fedora, RHEL, Alpine

Tizimingiz uchun paketni [soʻnggi reliz](https://github.com/nizom333/bdv/releases/latest) dan yuklab oling. Fayl nomlari (`VERSION` boshidagi `v` siz, masalan `0.1.0`):

| Format | amd64 | arm64 |
|---|---|---|
| deb | `bdv_VERSION_amd64.deb` | `bdv_VERSION_arm64.deb` |
| rpm | `bdv-VERSION-1.x86_64.rpm` | `bdv-VERSION-1.aarch64.rpm` |
| apk | `bdv_VERSION_x86_64.apk` | `bdv_VERSION_aarch64.apk` |

```sh
VERSION=0.1.0
curl -LO "https://github.com/nizom333/bdv/releases/download/v${VERSION}/bdv_${VERSION}_amd64.deb"
sudo apt install "./bdv_${VERSION}_amd64.deb"                 # Debian, Ubuntu
# sudo dnf install "./bdv-${VERSION}-1.x86_64.rpm"            # Fedora, RHEL
# sudo apk add --allow-untrusted "./bdv_${VERSION}_x86_64.apk" # Alpine
```

### Arch Linux (AUR)

Tez orada: `bdv-bin` (`yay -S bdv-bin`). Hozircha relizdagi binar fayldan yoki `go install` dan foydalaning.

### Go

```sh
go install github.com/nizom333/bdv@latest
```

`bdv` fayli `$(go env GOPATH)/bin` ga tushadi; `bdv --version` modul versiyasini koʻrsatadi.

### Binar fayllar

Har bir [reliz](https://github.com/nizom333/bdv/releases) da macOS, Linux va Windows uchun amd64 va arm64 arxivlari bor: `bdv_VERSION_OS_ARCH.tar.gz` (Windows uchun `.zip`), masalan `bdv_0.1.0_darwin_arm64.tar.gz`, hamda `checksums.txt`:

```sh
sha256sum --ignore-missing -c checksums.txt   # macOS: shasum -a 256 --ignore-missing -c checksums.txt
tar xzf bdv_0.1.0_linux_amd64.tar.gz && sudo mv bdv /usr/local/bin/
```

### Manba koddan

Go kerak (versiyasi `go.mod` da).

```sh
git clone https://github.com/nizom333/bdv && cd bdv
make install                     # ~/.local/bin ga
make install PREFIX=/usr/local   # boshqa joyga
```

Agar `~/.local/bin` `PATH` da boʻlmasa, `~/.zshrc` ga `export PATH="$HOME/.local/bin:$PATH"` qatorini qoʻshing.

## Foydalanish

```sh
cd <loyiha> && bdv          # .beads joriy papkada va undan yuqorida qidiriladi ($HOME gacha)
bdv <id>                    # darhol vazifada ochish
bdv --db <yoʻl>             # aniq baza (bd dagi kabi)
bdv --help | --version
```

Odatiy sxema: Claude Code kabi sunʼiy intellekt agenti bitta terminalda beads vazifalari ustida ishlaydi, bdv esa boshqasida daraxtni koʻrsatadi — agentdan soʻramasdan nima jarayonda, nima bloklangan va nima tugaganini koʻrasiz.

### Bazasiz loyiha

Agar beads bazasi hali yoʻq boʻlsa (joriy papkada va undan yuqorida `.beads` yoʻq yoki `bd` `no beads database found` deb javob bersa), bdv chiqib ketmaydi, balki uni yaratishni taklif qiladi: `i` ni bosing, tasdiqlashda papka va prefiksni tekshiring, soʻng `y` — joriy papkada bd ning standart sozlamalari bilan oddiy `bd init` ishga tushadi (prefiks — papka nomi). Tasdiqlashda ID rejimi ham koʻrinadi, `IDs: hash (bd default)`; `c` uni `IDs: sequential 1, 2, 3` ga va qaytib almashtiradi. Ketma-ket ID tanlanganda muvaffaqiyatli `bd init` dan keyin `bd config set issue_id_mode counter` bajariladi; bu buyruq muvaffaqiyatsiz boʻlsa, uning xatosi koʻrsatiladi, daraxt esa baribir ochiladi, chunki baza allaqachon bor. Boshqa har qanday tugma bekor qiladi. Muvaffaqiyatdan keyin boʻsh daraxt ochiladi. `~/.beads` — loyiha emas, bd ning global maʼlumotlar papkasi, shuning uchun bu yerda baza hisoblanmaydi. Taklif faqat `--db`, `BEADS_DIR` va `BEADS_DB` berilmagan boʻlsa chiqadi, aks holda baza qayerda boʻlishi kerakligi noaniq.

### Sozlamalar

`,` loyihaning bd sozlamalarini boʻlimlar boʻyicha ochadi: ID (prefiks, ID rejimi, xesh uzunliklari), statuslar va turlar (oʻz roʻyxatlari), vazifa yaratish (majburiy tavsif, tekshiruv, ichma-ichlik chuqurligi) va saqlash (Dolt avto-commit va avto-push, JSONL eksport, zaxira nusxalar, siqish). Qiymatlar `bd --readonly config list --json`, `bd --readonly config get <kalit> --json` va `bd --readonly types --json` orqali oʻqiladi; siz bermagan qiymat xira va `(default)` belgisi bilan koʻrsatiladi. `j k` tanlovni suradi, tanlangan sozlama ostidagi qator uni tavsiflaydi va `bd` kalitini aytadi, `Esc` daraxtga qaytaradi.

`Enter` (yoki `Space`) tanlangan qiymatni oʻzgartiradi: mantiqiy qiymatlar almashadi, variantlar aylanib oʻtadi, sonlar uchun diapazonni tekshiradigan maydon ochiladi, oʻz statuslari va turlari uchun esa muharrir ochiladi: `a` nom qoʻshadi (`a-z`, `0-9`, `_`), `x` (yoki `d`) tanlanganini oʻchiradi, `Enter` roʻyxatni saqlaydi; oʻrnatilgan statuslar va turlar koʻrinadi, lekin oʻchirilmaydi. `u` siz bergan qiymatni tiklaydi (`bd config unset`). Har bir oʻzgarish avval `<kalit>: <eski> → <yangi>` ni koʻrsatadi, xavfli kalitlar uchun ogohlantirish bilan (ID rejimi, Dolt avto-commit va avto-push, `error` tekshiruvi, vazifalar hali ishlatayotgan status yoki turni oʻchirish), va faqat `y` bilan bajariladi. ID prefiksi faqat oʻqish uchun: uni `bd rename-prefix` oʻzgartiradi. Maxfiy qiymatlar, `schema_version`, `db`, `actor`, `identity`, `sync.remote` va `federation.*` hech qachon koʻrsatilmaydi. `config.yaml` sozlamasi faqat `bd config get` uni fayl sozlamasi deb bildirsa (`location: config.yaml`) koʻrinadi; aks holda bu bd uni fayl sozlamasi deb hisoblamaydi va uni yozish bazada ishlatilmaydigan kalit qoldirardi, shuning uchun u yashirin (bd 0.59 da bu `export.auto`). Oʻz turlari har holda oʻqiladi. Agar baʼzi qiymatlar oʻqilmasa, qator nechtaligini aytadi, `r` ularni qayta oʻqiydi. bd bildirgan qiymatlar koʻrsatiladi, shuning uchun `BD_*` muhit oʻzgaruvchilari yoki global konfiguratsiya (`~/.config/bd/config.yaml`) loyiha faylidagini bekor qilishi mumkin.

### Varaqlar

`[` va `]` varaqlarni aylana boʻylab almashtiradi: `Tasks │ Specs N │ Decisions N │ Memory N │ Formulas N`. Faol varaq ajratib koʻrsatiladi, boʻshlari xira. Har bir varaq oʻz tanlovi, aylantirish holati va qidiruvini eslab qoladi; daraxtning ochilgan shoxlari va guruhlari oʻz holicha qoladi. `Tab`, kartochkani aylantirish, nusxalash, izohlar va `1`–`9` hamma joyda bir xil ishlaydi; `R`, `p` va `e` faqat Tasks da, `c` va `/` esa Tasks, Specs va Decisions da ishlaydi. Holat qatorida joriy varaqning tugmalari koʻrsatiladi.

```text
╭─ ◆ Specs (2) ─────────────────────────╮╭─ demo-app-k3f ──────────────────────────────────────────────╮
│ ◐ k3f Checkout flow       ██░░░░░░ 1/4││ Checkout flow                                               │
│ ○ p7a Product search      ░░░░░░░░ 0/2││  IN PROGRESS   P1  [epic]  ██░░░░░░ 1/4                     │
```

- **Tasks** — yuqorida tasvirlangan daraxt.
- **Specs** — shablonlardan (`template` belgili epiklardan) tashqari epiklar: to-spec koʻnikmasi spetsifikatsiyani shunday eʼlon qiladi — spetsifikatsiya epik tavsifida, tiketlar uning bolalari. Har bir qatorda status belgisi, ID, sarlavha va bevosita bolalar boʻyicha progress bor, `███░░░░░ 3/8` (hammasidan yopilganlari; hodisa yozuvlari hisobga olinmaydi). Kartochka tavsifni toʻliq, chiplar orasida progressni va barcha tiketlar hamda ularning statuslari bilan `Tickets` boʻlimini koʻrsatadi; `1`–`9` Tasks ga oʻtib, oʻsha tiketni tanlaydi. Ustuvorlik guruhlari bu yerda ishlamaydi.
- **Decisions** — `decision` turidagi vazifalar oddiy kartochka bilan tekis roʻyxatda.
- **Memory** — `bd memories` (chapda kalit, oʻngda markdown matn), yozuvlar boʻlmasa `No memories`.
- **Formulas** — `bd formula list` va shablonlar (`template` belgili epiklar va `bd list --include-templates` dagi bd shablon molekulalari): chapda nomlar, oʻngda tavsif. bdv formulalarni hech qachon ishga tushirmaydi.

Memory va Formulas varaq birinchi marta ochilganda va unda `r` bosilganda `bd --readonly memories --json`, `bd --readonly formula list --json` va `bd --readonly list --include-templates --flat --json --all -n 0` orqali oʻqiladi. Oʻqish xatosi panelda koʻrsatiladi; qolgan varaqlar ishlashda davom etadi.

## Tugmalar

| Qayerda | Tugmalar |
|---|---|
| Daraxt | `↑↓`/`j k` harakat · `→`/`l` ochish · `←`/`h` yopish yoki otaga oʻtish · `Enter`/`Space` ochish yoki yopish · `g`/`G` boshiga yoki oxiriga |
| Kartochka | `↑↓`/`j k` aylantirish · `PgUp`/`PgDn` sahifa · `1`–`9` havola boʻyicha oʻtish (istalgan fokusdan; Specs va Decisions dan Tasks ni ochadi) |
| Qidiruv | `/` ochish · yozish darhol filtrlaydi · `Enter` qadash · `Esc` tozalash |
| Izoh | `m` tanlangan vazifaga izoh yozish · `Ctrl+S` yuborish · `Esc` yopish, qoralama saqlanadi |
| Hamma joyda | `Tab` fokus · `[`/`]` varaqlar · `r` yangilash (Memory va Formulas da — varaqni qayta oʻqish) · `c` yopilganlar · `e` hodisalar · `p` ustuvorlik guruhlari · `R` faqat tayyorlar · `,` bd sozlamalari · `?` yordam · `q`/`Ctrl+C` chiqish |
| Sozlamalar | `,` ochish · `j k` harakat · `Enter`/`Space` oʻzgartirish · `u` standartga qaytarish · `y` tasdiqlash · `Esc` daraxtga qaytish |
| Nusxalash | `y` ID · `Y` sarlavha · `D` tavsif (xom markdown) · `M` kartochka markdown koʻrinishida (sarlavha, ID, status, ustuvorlik, tur, tavsif, eslatmalar) |
| Sichqoncha | gʻildirak kursorni suradi yoki aylantiradi · `S` sichqonchani ushlashni oʻchiradi va yoqadi |
| Tor rejim | `Enter` kartochkani ochish · `Esc` daraxtga qaytish |
| Bazasiz | `i` `bd init` ni taklif qilish · `c` ID ni almashtirish: xesh yoki ketma-ket · `y` ishga tushirish · `r` qayta urinish · `q` chiqish |

`?` xuddi shu roʻyxatni bdv ichida koʻrsatadi, `bdv --help` uni chop etadi.

Sichqoncha gʻildiragi kursor ostidagi panelni aylantiradi (daraxt ustida tanlovni suradi); bosishlar eʼtiborga olinmaydi.

Vazifa oʻzi yopilmagan boʻlsa va yopilmagan vazifaga `blocks` bogʻliqligi boʻlsa, u kutmoqda (bloklangan) hisoblanadi; `bd list` da yoʻq bloker ochiq deb hisoblanadi. Yopilgan vazifa blokerlari qanday boʻlmasin, hech qachon kutmaydi. Kutayotgan vazifalarning daraxtda sarlavhasi oldida qizil `⊘` boʻladi, yuqori qatorga `⊘ N waiting` qoʻshiladi (joriy `c`/`e` holatida koʻrinadigan vazifalar boʻyicha), kartochkada esa status chiplari ostida `⊘ waiting on N open blockers` yoziladi. `R` faqat ishga tayyor (bloklanmagan) vazifalarni qoldiradi; tayyor vazifalarning otalari xira kontekst sifatida qoladi. Filtr qidiruv, guruhlar va `c`/`e` bilan birga ishlaydi, holat qatorida `ready` chiqadi. Bloklangan vazifalar blokerlari ostiga joylanmaydi: bogʻlanishlar kartochkaning `Blocked by`/`Blocks` boʻlimlarida koʻrsatiladi.

Kartochka sarlavha, status chiplari, belgilar va sanalarni, soʻng mazmuni bor boʻlimlarni koʻrsatadi: `Description`, `Notes` va `bd show` dagi spetsifikatsiya maydonlari — `Spec` (`--spec-id` yoʻli yoki havolasi), `Design` va `Acceptance` (markdown), — ulardan keyin `Parent`, `Children`, `Blocked by`, `Blocks`, `Related` va `Comments`. Boʻlim nomlari barcha tillarda inglizcha qoladi.

Ustuvorlik guruhlari (`p`) ildiz vazifalarni `P0`…`P4` sarlavhalari ostiga yigʻadi. Sarlavha tanlanganda kartochka guruhdagi vazifalar sonini, statuslar boʻyicha xulosani va daraxt tartibida `[n] belgi id  sarlavha` koʻrinishidagi roʻyxat bilan `Tasks` boʻlimini koʻrsatadi (qizil `⊘` kutayotganlarini belgilaydi); roʻyxat `c`, `e`, `R` va qidiruvga moslashadi, `1`–`9` dastlabki toʻqqiztasiga oʻtadi.

Nusxalash matnni `bd` uni qanday saqlasa, shunday — markdown bilan birga — almashish buferiga qoʻyadi; faqat boshqaruv belgilari olib tashlanadi. Qalqib chiquvchi xabar nima nusxalanganini aytadi (`copied ID: …`, `copied title`, `copied description`, `copied card`) yoki tavsif boʻsh boʻlsa yoki guruh sarlavhasi tanlangan boʻlsa `nothing to copy`.

bdv sichqonchani ushlab turganda matnni belgilash uchun Shift ni bosib torting (Ghostty, iTerm2) yoki Fn ni bosib torting (Terminal.app). Yoki sichqonchani ushlashni oʻchirish uchun `S` ni bosing: terminalning oddiy belgilashi ishlaydi, gʻildirak endi panellarni aylantirmaydi, holat qatorida esa `S` ni yana bosmaguningizcha `mouse off` turadi. Ushlash oʻchiq paytda bdv terminalning alternate scroll rejimini (DECSET 1007) ham oʻchiradi, shunda gʻildirak kursorni suradigan strelkalarga aylanmaydi; u keyingi `S` da va chiqishda qayta yoqiladi.

## Sozlash va til

bdv ning oʻz konfiguratsiya fayli yoʻq. Bazani u `bd` kabi qidiradi: `--db <yoʻl>`, keyin `BEADS_DIR` / `BEADS_DB`, keyin joriy papkada va undan yuqorida `.beads`. Loyiha sozlamalari bd da saqlanadi va sozlamalar ekranidan (`,`) oʻzgartiriladi.

Interfeys standart holda inglizcha. Lokal `ru` bilan boshlansa, rus tiliga oʻtadi: `LC_ALL`, `LC_MESSAGES`, `LANG` dan birinchi boʻsh boʻlmagani olinadi. `BDV_LANG=en` yoki `BDV_LANG=ru` lokaldan ustun turadi.

```sh
BDV_LANG=ru bdv
```

## Nega bv emas?

[beads_viewer](https://github.com/Dicklesworthstone/beads_viewer) (`bv`) — beads uchun ajoyib va imkoniyatlarga boy koʻruvchi, graf tahlili va bdv’da hech qachon boʻlmaydigan darajada koʻp koʻrinishlarga ega. Agar ular sizga kerak boʻlsa — bv ni tanlang. bdv kichikroq maqsadni koʻzlaydi: bitta ekran, daraxt va kartochka, tez ishga tushish va yuqorida sanab oʻtilgan bir nechta tasdiqlanadigan yozuvlardan tashqari `bd` maʼlumotlariga qatʼiy faqat oʻqish uchun qarash. Uning joyi — agent yonidagi qoʻshni panelda, xalaqit bermasdan. Ikkalasi bemalol birga ishlaydi.

## Hissa qoʻshish

Xato haqidagi xabarlar, gʻoyalar va pull request’lar mamnuniyat bilan qabul qilinadi. Qanday yigʻish, tekshirish va oʻzgarishlarni yuborish — [CONTRIBUTING.md](CONTRIBUTING.md) da.

## Litsenziya

[MIT](LICENSE)
