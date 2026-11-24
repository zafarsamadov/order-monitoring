# CLAUDE.md — Order Integrity Monitor

DONA ombor stendidan (faqat o'qish) buyurtma, quti (package), handover va javonlarni tortib, o'z Postgres
bazamizda saqlaydigan va anomaliyalarni topadigan Go monolit. To'liq topshiriq: `task.md`.
Arxitektura shabloni: `../rider/rider-backend`.

> Bu fayl loyihadagi har bir muhim o'zgarish bilan yangilanadi (pastdagi "Holat" bo'limi).

## Ish tartibi
1. Bosqichlar qat'iy: 0 (API tahlili) → 1a (klient + sync, real data) → 1b (SQL tahlil, qoida nomzodlari) → 2.
   Har bosqich foydalanuvchi tasdig'idan keyin boshlanadi.
2. Qoidalar kodi faqat foydalanuvchi tasdiqlagan nomzodlar bo'yicha yoziladi.
3. Endpoint yo'li yoki maydon nomini o'ylab topma. Namunada (`apis/`, `docs/api-samples/`) yo'q bo'lsa — so'ra.
4. Docker fayllarini yozishdan oldin foydalanuvchidan shablon so'ra va uslubini saqla.
5. **Claude hech qachon commit qilmaydi** (git init ham foydalanuvchida). Bosqich oxirida kichik commitlar
   ro'yxati va xabarlarini taklif qiladi.

## Xavfsizlik (qat'iy)
- Stendga faqat `POST /auth/login` va `GET`. `dona` klientida faqat `Login` va `Get` metodlari bor;
  boshqa HTTP method yuboradigan kod yozilmaydi. Bu testda tekshiriladi.
- PVZ kassa va partners endpointlariga murojaat yo'q.
- Base URL, email, parol faqat `.env` dan. Kod, test, README, logga yozilmaydi; token/parol logda `***`.
- `.env`, `apis/` (xom namunalar, sirlar bor), `docs/api-samples/` (tozalangan nusxa) — `.gitignore`da.
- Bizning bazaga **faqat sync yozadi** (upsert + `sync_runs`). Sync hech qachon `DELETE`/`TRUNCATE` qilmaydi.
- Anomaliyalar, timeline, hisobot, CSV — faqat o'qiydi, `BEGIN READ ONLY` tranzaksiyada.
  Anomaliyalar jadvalga saqlanmaydi, har so'rovda hisoblanadi. `rules`/`report` paketlari `dona`ni import qilmaydi.

## Kod tuzilishi (rider-backend uslubi)
```
cmd/main.go            ishga tushirish: config → postgres (+migratsiya) → router
config/                env o'qish (majburiy kalit yo'q bo'lsa aniq xabar bilan to'xtaydi)
logger/                rangli logger (rider'dan)
models/                DTO va DONA javob tuzilmalari
dona/                  stend klienti: Login, Get, retry/backoff, 401 → qayta login
syncer/                sync orkestratori (stdlib `sync` bilan to'qnashmasligi uchun shu nom)
storage/               repo interfeyslari
storage/postgres/      pgxpool, upsert, read-only so'rovlar
migration/up|down      SQL migratsiyalar, ilova start'da embed orqali qo'llanadi (schema_migrations)
rules/                 har qoida alohida faylda + registry.go
report/                summary va CSV
handler/ api/          gin handlerlar va router
pkg/                   umumiy yordamchilar: hash.go, time.go (Sleep, Backoff), neterr.go, strings.go, sql.go, maps.go
web/                   bitta oddiy sahifa (go:embed): "Sinxronlash" va "Nomutanosibliklarni aniqlash"
```
Kutubxonalar: gin, pgx v4 (pgxpool), godotenv, cast. Ortiqcha bog'liqlik qo'shilmaydi.

## Qoidalar konvensiyasi
- `rules/<nom>.go`: tepada nomli konstantalar (threshold), `Name() Description() Severity() Check(ctx, store)`.
- Severity: `low | medium | high`. Anomaliya: qoida, severity, order code, qisqa sabab (qaysi maydon qanday qiymatda).
- Har qoida uchun table-driven test: bitta anomaliya holati va bitta normal holat.

## Sync konvensiyasi
- limit/offset pagination, `PAGE_SIZE` (50), `REQUEST_DELAY_MS`, `HTTP_TIMEOUT_SEC`, `MAX_RETRIES`.
- Retry: timeout, connection refused, 5xx, 429 → exponential backoff + jitter. 4xx retry yo'q. 401 → bir marta qayta login.
- Har sahifa bitta tranzaksiyada upsert, tabiiy kalit `id`. `raw jsonb` + `raw_hash`: hash teng bo'lsa faqat
  `last_synced_at` yangilanadi (`unchanged`), aks holda `updated`.
- `sync_runs`: holat, inserted/updated/unchanged, oxirgi offset, xato, API `total`. Bir vaqtda bitta sync (409).

## Kod uslubi
- Umumiy yordamchi funksiyalar `pkg/` da (rider uslubi, `package pkg`, mavzu bo'yicha alohida fayllar).
  Domen paketlarida faqat o'sha domenga xos kod qoladi (masalan `dona.isRetryable`).
- Sodda, o'qilishi oson. Izohlar o'zbekcha, qisqa, "nega"ni tushuntiradi.
- `docs/AI_NOTES.md` har bosqichda yangilanadi: Claude nima yozdi, foydalanuvchi nimani o'zgartirdi/tashladi,
  qayerda xato bo'ldi, ketgan vaqt.

## Holat
- **0-bosqich** tugadi: API tahlili (endpointlar, maydonlar, bog'lanishlar) — reja faylida.
- **1a.0**: CLAUDE.md, .gitignore, .env.example, docs/AI_NOTES.md, docs/api-samples/ (tozalangan) yaratildi.
- **1a (kod)**: skeleton, config, migratsiya (00001: orders, packages, handovers, shelves, shelf_tiles,
  sync_runs), `dona` klient, `syncer`, `POST /sync`, `GET /sync/status`, Makefile. Testlar o'tadi.
- **Frontend (sync qismi)**: `web/` (index.html, style.css, app.js, embed.go), `GET /` va `/static/*`.
  Faqat "Sinxronlash" tugmasi; anomaliyalar va CSV keyin.
- **Real sync** (2026-11-24): orders 638/638, packages 16/16 (filtrsiz `/packages?warehouse=` ishlaydi),
  handovers 39/39, shelves+tiles 16. Ikkinchi sync: 0 yangi, 0 yangilangan, 709 o'zgarmagan.
  Keyingi qadam: SQL bilan ma'lumot tahlili → qoida nomzodlari (1b).

## Buyruqlar
- `make test-db-up && make test` — testlar (sync testlari Postgres'siz skip bo'ladi), `make test-db-down`.
- `make run` — ilova (`.env` kerak).

## Muhim detallar
- Upsert: `storage/postgres/sync.go` `upsert()` — `ON CONFLICT ... WHERE raw_hash <> EXCLUDED.raw_hash RETURNING (xmax = 0)`.
  Qator qaytmasa → unchanged, faqat `last_synced_at` yangilanadi.
- Hash kanonik JSON'dan (kalitlar saralangan). Javonlarning `oldest_age_seconds`, `progress` maydonlari hash'ga
  kirmaydi (har so'rovda o'zgaradi). Tile hash'iga ota javon kodi qo'shiladi.
- Bo'sh satr ustunlarda NULL bo'ladi (`pkg.EmptyToNil`) — qoidalarda "yo'q"ni `IS NULL` bilan tekshiring.
- Baza: `.env` da alohida `POSTGRES_HOST/PORT/USER/PASSWORD/DATABASE` (URL emas); DSN `config.PostgresURL()`.
  Testlar esa bitta `TEST_DATABASE_URL` oladi (Makefile'da sukut qiymati bor).
- Stendga ulanish uchun VS Code'da macOS Local Network ruxsati kerak (aks holda `no route to host`).
  `DONA_API_BASE_URL` `/api/v1` bilan tugaydi.
- `NetError.Error()` stend host'ini chiqarmaydi (`url.Error` ichidagi manzil logga tushmasligi uchun).
