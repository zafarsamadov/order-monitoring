# Order Integrity Monitor

DONA ombor stendidan buyurtmalar (orders), qutilar (packages), handover'lar va javonlarni o'qib, o'z Postgres
bazasiga saqlaydigan va ulardagi nomutanosibliklarni (anomaliyalarni) topadigan Go servis.

- Stendga faqat o'qish uchun murojaat qilinadi: `POST /auth/login` va `GET` so'rovlari.
- Bizning bazaga faqat sync yozadi. Anomaliyalar, timeline va hisobot faqat o'qiydi.

## Mundarija
1. [Ishga tushirish](#ishga-tushirish)
2. [Sync](#sync)
3. [Domen: nima normal, nima xato](#domen-nima-normal-nima-xato)
4. [API va hisobot](#api-va-hisobot)
5. [Testlar](#testlar)
6. [AI bilan ishlash](#ai-bilan-ishlash)

---

## Ishga tushirish

Talablar: Go 1.26+, Postgres 14+, testlar uchun Docker.

```bash
cp .env.example .env      # DONA_* va POSTGRES_PASSWORD ni to'ldiring
make migrate              # faqat migratsiyalar (ilova start'ida ham avtomatik qo'llanadi)
make run                  # server: http://localhost:8080
```

Brauzerda `http://localhost:8080` sahifasi ochiladi. Unda ikkita tugma bor: **Sinxronlash** va
**Nomutanosibliklarni aniqlash**.

| O'zgaruvchi | Sukut | Ma'nosi |
|---|---|---|
| `DONA_API_BASE_URL` | — | stend manzili, `/api/v1` bilan tugaydi |
| `DONA_EMAIL`, `DONA_PASSWORD` | — | stend login ma'lumotlari |
| `DONA_WAREHOUSE_CODE` | bo'sh | bo'sh qoldiring (pastda [nega](#packages-filtri)) |
| `POSTGRES_HOST/PORT/USER/PASSWORD/DATABASE` | localhost, 5432, postgres, —, order_monitoring | baza |
| `PAGE_SIZE` | 50 | bitta sahifada nechta yozuv so'raladi |
| `REQUEST_DELAY_MS` | 300 | so'rovlar orasidagi pauza (stendni yuklamaslik uchun) |
| `HTTP_TIMEOUT_SEC` | 15 | bitta so'rov uchun timeout |
| `MAX_RETRIES` | 5 | vaqtinchalik xatoda qayta urinishlar soni |
| `SYNC_ON_START` | false | ilova start'ida avtomatik sync |

Sirlar (`.env`) va xom API namunalari (`apis/`) `.gitignore` da turadi va repoga tushmaydi.

---

## Sync

`POST /sync` (yoki sahifadagi **Sinxronlash** tugmasi) quyidagi tartibda ishlaydi:

| Obyekt | Endpoint | Usul |
|---|---|---|
| orders | `GET /orders` | limit/offset pagination |
| packages | `GET /packages` | limit/offset pagination |
| handovers | `GET /inbound/handovers` | limit/offset pagination |
| shelves + tiles | `GET /shelves` | bitta snapshot (pagination yo'q) |

Natijada har bir obyekt uchun quyidagilar ko'rsatiladi: nechta olindi, API `total`, nechtasi yangi, yangilangan,
o'zgarmagan va o'tkazib yuborilgan.

### Pagination
- Har bir so'rov `limit=PAGE_SIZE&offset=N` bilan yuboriladi. `offset` olingan yozuvlar soniga oshiriladi.
- Sync sahifa bo'sh kelganda yoki `offset >= total` bo'lganda to'xtaydi.
- Qisqa sahifa (`PAGE_SIZE` dan kam yozuv) tugash belgisi hisoblanmaydi, chunki server `limit` ni o'zi kesishi
  mumkin. Masalan, 50 so'ralsa 20 qaytarishi mumkin. Qisqa sahifa faqat API `total` bermagan holatda tugash
  belgisi bo'ladi.
- So'rovlar orasida `REQUEST_DELAY_MS` pauza qilinadi.

### Dublikat yo'qligi (takroriy sync)
- Har bir yozuvning tabiiy kaliti stenddagi `id` (UUID). `order_code` ko'p orderlarda bo'sh, shuning uchun kalit
  sifatida ishlatilmaydi.
- Yozuv `INSERT ... ON CONFLICT (id) DO UPDATE ... WHERE raw_hash <> EXCLUDED.raw_hash` bilan saqlanadi.
  - `raw_hash` — kalitlari saralangan kanonik JSON'ning sha256 qiymati.
  - Hash o'zgarmagan bo'lsa yozuv **unchanged** hisoblanadi va faqat `last_synced_at` yangilanadi.
  - Hash o'zgargan bo'lsa yozuv **updated** bo'ladi; `first_seen_at` saqlanib qoladi.
- Javonlardagi har so'rovda o'zgaradigan maydonlar (`oldest_age_seconds`, `progress`) hash'ga kirmaydi. Aks holda
  har bir sync ularni "yangilangan" deb ko'rsatgan bo'lardi.
- Sync hech narsani o'chirmaydi (`DELETE`/`TRUNCATE` yo'q).
- Real stendda takroriy sync natijasi: 0 yangi, 0 yangilangan, hammasi o'zgarmagan.

### Tarmoq xatolari
Stend klientining (`dona/`) ochiq metodlari faqat `Login` va `Get`. Boshqa HTTP metod yuborish imkoni yo'q, va bu
testda tekshiriladi.

| Holat | Xatti-harakat |
|---|---|
| timeout, connection refused/reset, javob o'rtasida uzilish | qayta urinish |
| 5xx | qayta urinish |
| 429 / 503 + `Retry-After` | server aytgan vaqt kutiladi (eng ko'pi 10s), keyin qayta urinish |
| 200, lekin tanasi buzilgan yoki HTML | qayta urinish |
| 401 | bir marta qayta login, keyin so'rov takrorlanadi |
| boshqa 4xx | qayta urinilmaydi, xato qaytadi |

- Qayta urinishlar exponential backoff (0.5s, 1s, 2s, … 10s gacha) va jitter bilan bajariladi. Urinishlar soni
  `MAX_RETRIES` bilan belgilanadi.
- Xato xabarlarida stend manzili chiqmaydi. Token va parol logga yozilmaydi.

### Uzilish va qayta ishga tushirish
- Har bir sahifa va `sync_runs` dagi run holati bitta tranzaksiyada saqlanadi. Sync o'rtada uzilsa, saqlangan
  sahifalar bazada qoladi va run `failed` statusini oladi; xato matni ham saqlanadi.
- Keyingi sync boshidan yuradi. Saqlangan yozuvlar `unchanged` bo'ladi, qolganlari qo'shiladi, dublikat paydo
  bo'lmaydi.
- Ilova sync o'rtasida o'chirilsa, keyingi start'da `running` holatida qolgan run `failed` bo'ladi va xatosiga
  "sync uzilib qolgan" deb yoziladi.
- Bir vaqtda faqat bitta sync ishlaydi. Ikkinchi so'rov `409 Conflict` oladi.

### `/packages` filtri
`/packages?warehouse=WH-TAS-01` faqat 16 ta quti qaytaradi, filtrsiz so'rov esa 641 ta. Orderlar ombor bo'yicha
filtrlanmaydi, qoidalar esa qutini orderga bog'lab tekshiradi. Shuning uchun qutilar ham filtrsiz olinadi
(`DONA_WAREHOUSE_CODE` bo'sh).

### Ma'lum cheklov
Sync paytida stendga yangi yozuv qo'shilsa, offset siljiydi va sahifalar chegarasidagi bitta yozuv o'tkazib
yuborilishi mumkin. Buni murakkab kod bilan hal qilmadik, chunki:
- upsert dublikat bermaydi;
- keyingi sync o'tkazib yuborilgan yozuvni oladi;
- olingan yozuvlar soni API `total` ga teng bo'lmasa, logga ogohlantirish yoziladi.

Real sync natijasi (2026-11-24): orders 638/638, packages 641/641, handovers 39/39, javonlar 10.

---

## Domen: nima normal, nima xato

Obyektlar: **order** (buyurtma), **package** (quti), **handover** (sotuvchi tovarini omborga topshirishi),
**shelf** (javon). Bog'lanish: `order.id = package.order_group_id = handover.order_group_id`.

Topilgan holatlar 3 guruhga ajratilgan:

| Guruh | Nima qilindi |
|---|---|
| [Aniq xato](#1-aniq-xatolar) — ma'lumot o'ziga zid | qoida yozildi |
| [Shubhali](#2-shubhali-otkazib-yuborildi) — xato ham, normal ham bo'lishi mumkin | qoida yozilmadi, sababi pastda |
| [G'alati, lekin normal](#3-galati-lekin-normal) | qoida yozilmadi |

Sonlar — real sync natijasi (2026-11-24: 638 order, 641 quti, 39 handover, 10 javon).
Qoidalar kodi: `storage/postgres/anomalies/<qoida_nomi>.go`.

### 1. Aniq xatolar

Har bir qoidada ikki maydon (yoki ikki yozuv) bir-biriga zid: ikkalasi bir vaqtda to'g'ri bo'la olmaydi.

**`issued_order_package_returned`** · high · 26 ta order
Order xaridorga topshirilgan (`lifecycle=OUT`, `ISSUED_TO_BUYER`), lekin qutilaridan biri `return_id` bilan omborga
qaytgan (`RETURNED_TO_WAREHOUSE`).
*Nega xato:* quti qaytganda order `RETURN` bosqichiga o'tadi — 216 ta qaytgan orderda aynan shunday. Bu yerda order
holati qaytishni "ko'rmagan".
*Misol:* GRP-231612 — PKG-RPT-810093 RETURNED_TO_WAREHOUSE, order hali READY_TO_SHIP.

**`conflicting_package_statuses`** · medium · 26 ta order
Order `RETURN_TO_HUB` ("kuryer qaytarishni olib kelyapti"), lekin qaytarilayotgan quti allaqachon
`RETURNED_TO_SELLERS`.
*Nega xato:* qaytish tartibi xaridor → hub → sotuvchi. Quti hali hub'ga yetmagan bo'lsa, sotuvchida bo'la olmaydi.
*Misol:* GRP-428899 — PKG-RPT-046434 RETURNED_TO_SELLERS.

**`returned_to_seller_in_warehouse`** · medium · 210 ta quti
Quti sotuvchiga qaytarilgan (`RETURNED_TO_SELLERS`), lekin `shelf_code` hali javonni ko'rsatadi.
*Nega xato:* javonning o'zi bu qutini sanamaydi — RET-01 da `box_count = 6`, bu faqat RETURNED_TO_WAREHOUSE qutilar.
`shelf_code` eskirib qolgan.
*Misol:* QV1794375397IV — PKG-9364 RET-01.

**`package_on_unknown_shelf`** · high · 2 ta quti
Omborda turgan quti (`RETURNED_TO_WAREHOUSE`) javonlar ro'yxatida yo'q javonda.
*Nega xato:* `shelf_code` `/shelves` ro'yxatidagi hech bir javonga mos kelmaydi — quti omborda, lekin uni topib
bo'lmaydi. Ombordan chiqib ketgan qutilar bu yerda tekshirilmaydi (ular yuqoridagi qoidada).
*Misol:* GRP-216217 — PKG-9389 RET-T.

**`dispatched_package_on_shelf`** · medium · 1 ta quti
Quti jo'natilgan (`DISPATCHED`), lekin hali javonda.
*Nega xato:* javon 0 ning `box_count = 3`, unda turibdi deb yozilgan qutilar esa 4 ta — ortiqchasi shu jo'natilgan quti.
*Misol:* HC1794401226GR — PKG-10192, javon 0.

**`handover_timestamps_out_of_order`** · high · 22 ta handover
Handover vaqtlari tartibi buzilgan: `created_at ≤ transit_since ≤ received_at` bo'lishi kerak.
*Nega xato:* yaratilmagan handover yo'lga chiqa yoki qabul qilina olmaydi.
9 tasida qabul qilish yaratilishdan oldin (13 kungacha), 14 tasida yo'lga chiqish yaratilishdan aynan 2 soat oldin —
bu vaqt zonasi xatosiga o'xshaydi.
*Misol:* HND-TESTPM-w17 — received_at created_at dan 13 kun oldin.

**`activity_before_lifecycle_start`** · medium · 3 ta order
Orderning oxirgi faolligi joriy bosqich boshlanishidan oldin.
*Nega xato:* bosqich o'zgarishining o'zi faollik, demak `last_activity_at ≥ lifecycle_since`. 1 daqiqagacha farq
o'tkaziladi (yozish kechikishi).
*Misol:* GZ1794375397MA — 12 kun oldin.

**`handover_received_incomplete`** · medium · 3 ta handover
Handover qabul qilingan (`RECEIVED`), lekin kelgan tovar (`item_count`) kutilganidan (`expected_qty`) kam.
*Nega xato:* `RECEIVED` — qabul yakunlangan degani, kamomad bilan yopilgan. Stendda qisman qabul uchun alohida holat
(masalan PARTIAL) yo'q, shuning uchun kamomad faqat shu yo'l bilan ko'rinadi. Yo'ldagi (`IN_TRANSIT`) handover'da
kamomad normal — qism-qism kelishi mumkin, u tekshirilmaydi.
*Misol:* EO1794401263JL — HND-2607-T, kelgan 1 / kutilgan 2.

**`order_item_count_mismatch`** · low · 0 ta order
Orderdagi tovar soni qutilardagi tovarlar yig'indisiga teng emas.
*Nega xato:* qutilar orderning o'zidan yig'iladi, sonlar mos kelishi shart.
Faqat `item_count > 0` bo'lgan orderlar tekshiriladi: stend bu maydonni 638 ta orderdan faqat 39 tasida to'ldiradi,
0 — "noma'lum". Hozirgi ma'lumotda nomutanosiblik yo'q; maydon to'ldirilsa, qoida ishlaydi.

### 2. Shubhali (o'tkazib yuborildi)

Xato bo'lishi mumkin, lekin ma'lumotning o'zida ziddiyat yo'q — faqat taxmin. Qoida qilinmadi: yolg'on signal
berish xavfi katta.

**Handover uzoq vaqt yo'lda (`IN_TRANSIT`)** · 11 ta handover, 14 kungacha
*Nega xato bo'lishi mumkin:* yuk yo'qolgan yoki qabul qilish belgilanmay qolgan bo'lishi mumkin.
*Nega qoida qilinmadi:* ma'lumot o'ziga zid emas — handover hali yo'lda deyilgan va yo'lda bo'lishi mumkin.
"Necha kun ko'p?" chegarasi (masalan 3 kun) taxmin, uni biznes belgilaydi. Bundan tashqari stend ma'lumoti
2026-11-11 dan beri yangilanmagan: `now()` ga nisbatan hisoblasak, vaqt o'tgan sari har bir yo'ldagi handover
avtomatik "tiqilib qolgan" bo'lib chiqadi. 5 tasida `transit_since` umuman yo'q.
*Misol:* HND-TESTPM-h15 — 2026-11-11 dan beri IN_TRANSIT.

### 3. G'alati, lekin normal

**Bitta javonda bir nechta orderning qutilari**
*Nega g'alati ko'rinadi:* "bir javon — bir order" deb kutish mumkin.
*Nega normal:* javon — saralash joyi, order uchun ajratilgan joy emas. Har bir quti o'z `shelf_code` iga ega, va
javonning `box_count` i shu qutilar soniga teng bo'lsa, ma'lumot to'g'ri.
Real ma'lumotda: javon 0 — 3 ta orderdan 3 quti, javon 1 — 3 ta orderdan 3 quti, RET-01 — 6 ta orderdan 6 quti.

> TODO: qolgan normal holatlar.

---

## API va hisobot

| Endpoint | Ma'nosi |
|---|---|
| `POST /sync` | sync'ni ishga tushiradi; tugaguncha kutadi va natijani qaytaradi (409 — sync allaqachon ketyapti) |
| `GET /sync/status` | oxirgi sync natijasi va hozir sync ketyaptimi |
| `GET /anomalies?rule=&severity=` | anomaliyalar ro'yxati; ikkala filtr ixtiyoriy, `severity`: `low`, `medium`, `high` |
| `GET /orders/{code}/timeline` | orderning hodisalar tarixi; `code` o'rniga `order_code`, `group_no` yoki `id` berish mumkin; topilmasa 404 |

> TODO: hisobot (HTML/CSV).

---

## Testlar

Sync va storage testlari real Postgres talab qiladi. `TEST_DATABASE_URL` berilmasa, ular **skip** bo'ladi. To'liq
testlash uchun:

```bash
make test-db-up     # vaqtinchalik postgres:17-alpine konteyneri, port 55432
make test           # TEST_DATABASE_URL bilan go test ./...
make test-db-down
```

Har bir test o'z Postgres sxemasida ishlaydi va oxirida o'sha sxemani o'chiradi.

| Soha | Testlar |
|---|---|
| Stend klienti | 500 → retry; 429 → retry, keyin to'xtash; 4xx retry yo'q; 401 → qayta login; connection refused (host yashirin); `Retry-After`; buzilgan JSON; faqat GET + login yuboriladi; ochiq metodlar faqat `Login`/`Get` |
| Sync | pagination + 500 dan keyin retry + takroriy sync dublikatsiz + o'zgargan yozuv yangilanadi; server limit'ni kesadi; o'rtada uzilish va davom etish; parallel sync rad etiladi |
| Timeline | hodisalar tartibi, bo'sh vaqtlar, 404, handler javoblari |

> TODO: qoidalar testlari.

---

## AI bilan ishlash

> TODO: AI (Claude Code) nima qildi, nimani men o'zgartirdim yoki tashladim, AI qayerda xato qildi.
> Batafsil qaydlar: `docs/AI_NOTES.md`.
