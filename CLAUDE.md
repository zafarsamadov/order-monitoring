# CLAUDE.md — Order Integrity Monitor

Go monolit. DONA ombor stendidan (faqat o'qish) buyurtma, quti (package), handover va javonlarni tortib,
o'z Postgres bazamizga saqlaydi va anomaliyalarni topadi. Foydalanuvchi uchun hujjat: `README.md`.

> Bu fayl loyihadagi har bir muhim o'zgarish bilan yangilanadi.

## Ish tartibi
1. Katta o'zgarishdan oldin reja tuziladi va foydalanuvchi tasdig'idan keyin bajariladi.
2. Endpoint yo'li yoki maydon nomini o'ylab topma. Namunada (`docs/api-samples/`) yo'q bo'lsa — so'ra.
3. Docker fayllarini yozishdan oldin foydalanuvchidan shablon so'ra.
4. **Claude commit qilmaydi.** Ish oxirida commit xabarini taklif qiladi.
5. Jarayon yoki konteyner ishga tushirilsa — oldindan aytiladi va ish tugagach to'xtatiladi.

## Xavfsizlik (qat'iy)
- Stendga faqat `POST /auth/login` va `GET`. `dona` klientida faqat `Login` va `Get` metodlari bor;
  boshqa HTTP method yuboradigan kod yozilmaydi. Bu testda tekshiriladi.
- PVZ kassa va partners endpointlariga murojaat yo'q.
- Base URL, email, parol faqat `.env` dan. Kod, test, README, logga yozilmaydi; token/parol logda `***`.
- `.env`, `apis/` (xom namunalar, sirlar bor), `docs/` (tozalangan namunalar) — `.gitignore`da.
- Bizning bazaga **faqat sync yozadi** (upsert + `sync_runs`). Sync hech qachon `DELETE`/`TRUNCATE` qilmaydi.
- Anomaliyalar, timeline, hisobot faqat `SELECT` qiladi. Anomaliyalar jadvalga saqlanmaydi, har so'rovda hisoblanadi.

## Kod tuzilishi
Qatlamlar: `handler` → `service` → `storage` (repo). Handler faqat so'rov/javob bilan ishlaydi, mantiq service'da.
```
cmd/main.go                    ishga tushirish: config → postgres (+migratsiya) → router; -migrate flag
config/                        env o'qish (majburiy kalit yo'q bo'lsa aniq xabar bilan to'xtaydi)
logger/                        rangli logger
models/                        DTO, DONA javob tuzilmalari, timeline va anomaliya modellari
dona/                          stend klienti: Login, Get, retry/backoff, Retry-After, 401 → qayta login
service/                       sync (sync.go, sync_records.go), timeline (order.go), anomaliyalar (anomaly.go)
storage/storage.go             repo interfeyslari
storage/postgres/              pgxpool, upsert (sync.go), timeline so'rovlari (order.go)
storage/postgres/anomalies/    har qoida alohida faylda; consts.go — umumiy statuslar va severity; repo.go
migration/up|down              SQL migratsiyalar, ilova start'ida embed orqali qo'llanadi (schema_migrations)
handler/ api/                  gin handlerlar va router
pkg/                           umumiy yordamchilar: hash, time (Sleep, Backoff), neterr, strings, sql, maps
web/                           bitta sahifa (go:embed): sync va anomaliyalar
```
Kutubxonalar: gin, pgx v4 (pgxpool), godotenv, cast. Ortiqcha bog'liqlik qo'shilmaydi.

## Qoidalar konvensiyasi (`storage/postgres/anomalies/`)
- Fayl nomi = qoida nomi (`handover_received_incomplete.go`). Tepada izoh `// N-qoida: ...` va konstantalar:
  `xxxRule`, `xxxDescription`, `xxxSeverity` (+ threshold bo'lsa). Bir nechta qoida ishlatadigan qiymat — `consts.go`.
- Struct `Xxx`, konstruktor `NewXxxRepo(db)`, metod `XxxRule(ctx) (*models.RuleItem, error)`.
  Natija bo'lmasa bo'sh `Items` qaytadi (nil emas). `rows.Err()` va `resp.Total` majburiy.
- Yangi qoida: `repo.go` ga embed, `storage.AnomaliesRepoI` ga metod, `service/anomaly.go` ro'yxatiga qo'shiladi.
- Severity: `low | medium | high`. Sabab (`Reason`): qaysi maydon qanday qiymatda.

## Sync konvensiyasi
- limit/offset pagination, `PAGE_SIZE` (50), `REQUEST_DELAY_MS`, `HTTP_TIMEOUT_SEC`, `MAX_RETRIES`.
- To'xtash: bo'sh sahifa yoki `offset >= total`. Qisqa sahifa faqat `total` yo'q bo'lsa tugash belgisi.
- Retry: timeout, connection refused, 5xx, 429, buzilgan JSON → exponential backoff + jitter; `Retry-After` bo'lsa
  shuncha (`maxBackoff` gacha). 4xx retry yo'q. 401 → bir marta qayta login.
- Har sahifa bitta tranzaksiyada upsert, tabiiy kalit `id`. `raw jsonb` + `raw_hash`: hash teng bo'lsa faqat
  `last_synced_at` yangilanadi (`unchanged`), aks holda `updated`.
- `sync_runs`: holat, inserted/updated/unchanged, oxirgi offset, xato, API `total`. Bir vaqtda bitta sync (409).
- `/packages` filtrsiz olinadi (`DONA_WAREHOUSE_CODE` bo'sh): `warehouse=` bilan faqat 16 ta keladi, orderlar esa
  filtrlanmaydi va qoidalar qutini orderga bog'laydi.

## Kod uslubi
- Sodda, o'qilishi oson; mayda bir-birini chaqiradigan funksiyalar ko'paytirilmaydi.
- Modellar `models/` da, umumiy yordamchilar `pkg/` da. Domen paketida faqat o'sha domenga xos kod.
- SQL: `query := \`...\`` ko'p qatorli, ustunlar alohida qatorda, `rows.Scan(` argumentlari ham alohida qatorda.
- Izohlar o'zbekcha, qisqa, "nega"ni tushuntiradi.
- `docs/AI_NOTES.md`: Claude nima yozdi, foydalanuvchi nimani o'zgartirdi/tashladi, qayerda xato bo'ldi.

## Holat (2026-11-25)
- Sync tayyor va testlangan. Real sync: orders 638, packages 641, handovers 39, javonlar 10.
  Takroriy sync: 0 yangi, 0 yangilangan.
- API: `POST /sync`, `GET /sync/status`, `GET /orders/{code}/timeline`, `GET /anomalies?rule=&severity=`.
- Qoidalar: 9 ta, faqat aniq xatolar (ma'lumot o'ziga zid). Shubhali holatlar qoida qilinmaydi — README'da
  "Shubhali" bo'limida sababi bilan yoziladi. Ta'riflar va dalillar: README "Domen" bo'limi.
- Qolgan ishlar: qoidalar testlari, HTML/CSV hisobot, timeline'ga anomaliyalarni ulash, README'ning TODO bo'limlari.
- Ma'lum cheklov: sync paytida stendga yangi yozuv qo'shilsa, offset siljib bitta yozuv o'tkazib yuborilishi mumkin;
  keyingi sync uni oladi.

## Buyruqlar
- `make run` — ilova (`.env` kerak); `make migrate` — faqat migratsiyalar.
- `make test-db-up && make test && make test-db-down` — testlar (Postgres testlari `TEST_DATABASE_URL` siz skip).

## Muhim detallar
- Upsert: `storage/postgres/sync.go` `upsert()` — `ON CONFLICT ... WHERE raw_hash <> EXCLUDED.raw_hash RETURNING (xmax = 0)`.
  Qator qaytmasa → unchanged, faqat `last_synced_at` yangilanadi.
- Hash kanonik JSON'dan (kalitlar saralangan). Javonlarning `oldest_age_seconds`, `progress` maydonlari hash'ga
  kirmaydi (har so'rovda o'zgaradi). Tile hash'iga ota javon kodi qo'shiladi.
- Bo'sh satr ustunlarda NULL bo'ladi (`pkg.EmptyToNil`) — qoidalarda "yo'q"ni `IS NULL` bilan tekshiring.
- `orders.item_count` 638 tadan 599 tasida 0 — "noma'lum" ma'nosida; qoidalarda buni hisobga oling.
- Baza: `.env` da alohida `POSTGRES_HOST/PORT/USER/PASSWORD/DATABASE` (URL emas); DSN `config.PostgresURL()`.
  Testlar esa bitta `TEST_DATABASE_URL` oladi (Makefile'da sukut qiymati bor).
- Stendga ulanish uchun macOS'da IDE'ga Local Network ruxsati kerak (aks holda `no route to host`).
  `DONA_API_BASE_URL` `/api/v1` bilan tugaydi.
- Migratsiya `pg_advisory_lock` ostida: parallel start/testlarda to'qnashmaydi.
- Store repolari konstruktorda yaratiladi (lazy init parallel so'rovlarda data race berardi).
