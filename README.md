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

**Jo'natishga tayyor order, lekin bitta ham qutisi yo'q** · 7 ta order
*Nega xato bo'lishi mumkin:* order `SORTING / READY_TO_SHIP` ("Ready to ship · waiting for the courier"), lekin
`/packages` da unga tegishli birorta quti yo'q — jo'natadigan narsa ko'rinmaydi.
*Nega qoida qilinmadi:* ziddiyat bizning bazamizda, stendning o'zida emas bo'lishi mumkin: `/packages` hamma qutini
qaytarmagan bo'lishi mumkin (masalan `warehouse=` filtri bilan faqat 16 ta keladi). Qutining yo'qligi uning
mavjud emasligini isbotlamaydi.
*Misol:* LS1794375397FV, WR1794375397AP — READY_TO_SHIP, qutisi yo'q.

**Qayta qadoqlangan quti (`REPACKED`) qayerda ekani noma'lum** · 26 ta quti
*Nega xato bo'lishi mumkin:* qutida na `held_since` (kimda/qachondan beri), na `shelf_code` bor — u qayerda ekanini
aytib bo'lmaydi.
*Nega qoida qilinmadi:* qayta qadoqlash endi boshlangan va quti hali joylashtirilmagan bo'lishi mumkin. Hammasi
qaytish yo'lidagi orderlarda (`RETURN_TO_HUB`) — jarayonning oraliq holati.
*Misol:* GRP-428899 — PKG-RPK-051890 REPACKED.

### 3. G'alati, lekin normal

Bular xatoga o'xshaydi, lekin biznes jarayoni yoki stend ma'lumotining shakli bilan tushuntiriladi. Qoida qilinmadi;
ba'zilari qoidalar testlarida "normal" qator sifatida turibdi — qoida ularni xato deb chiqarmasligi tekshiriladi.

**Javonlar va qutilar**

| Holat | Soni | Nega normal |
|---|---|---|
| Bitta javonda bir nechta orderning qutilari | javon 0: 3, javon 1: 3, RET-01: 6 order | Javon — saralash joyi, orderga ajratilmagan. Javonning `box_count` i qutilar soniga teng — ma'lumot to'g'ri |
| Omborga qaytgan quti (`RETURNED_TO_WAREHOUSE`) RET-01 javonida | 6 | Qaytarishlar javoni aynan shu uchun. RET-01 `box_count = 6` — hammasi sanalgan |
| Xaridorga topshirilgan yoki sotuvchiga qaytarilgan quti javonsiz | 52 + 104 | Quti ombordan chiqib ketgan — javonda bo'lmasligi kerak |
| Bitta orderda 2–3 ta quti | 52 order (masalan GRP-428899 — 3 ta) | Order bir nechta qutiga bo'lib qadoqlanadi |
| Order `AWAITING` / `ACCEPTED`, lekin qutisi yo'q | 63 + 5 | Tovar hali omborga kelmagan yoki qadoqlanmagan; quti qadoqlashda yaratiladi (masalan DD1794401226LS) |
| Maxsus javonlarda (RET-01, RTY-01, …) `tiles` bo'sh, lekin `box_count > 0` | RET-01: 6 | `/shelves` faqat saralash javonlarining qutilarini beradi; maxsus javonlardagilar `/packages` da `shelf_code` bilan ko'rinadi |

**Qaytish jarayoni**

| Holat | Soni | Nega normal |
|---|---|---|
| Bitta orderda ham `ISSUED_TO_BUYER`, ham `RETURNED_TO_SELLERS` quti | — | Qisman qaytish: xaridor bitta qutini oldi, ikkinchisini qaytardi. Holatning o'zi xato emas (bazadagi shunday 26 ta order `conflicting_package_statuses` ga boshqa sabab bilan tushadi: kuryer hali olib kelyapti, quti esa allaqachon sotuvchida) |
| RETURN orderlarda `location = STAGING` | 531 dan 530 | Stend modeli: joylashuv qaytish bosqichi bilan bir vaqtda yoziladi (`location_since = lifecycle_since`). Barcha qaytishlarda bir xil — tizimli, xato emas |
| `RETURN_TO_HUB` + `READY_TO_SHIP` ("Courier bringing a return") | 27 | Qaytish yo'lda: tovar hali hub'ga kelmagan |

**Handover'lar**

| Holat | Soni | Nega normal |
|---|---|---|
| Yo'ldagi (`IN_TRANSIT`) handover'da `item_count < expected_qty` | 2 (HND-2601-T 2/4, HND-2603-T 2/3) | Tovar qism-qism yuborilmoqda; kamomad faqat qabul qilingandan (`RECEIVED`) keyin xato |
| Orderda handover yo'q | 599 order | `/inbound/handovers` jami 39 ta yozuv qaytaradi — oxirgi davr. Handover'ning yo'qligi xato emas |

**Vaqtlar, to'lov va bo'sh maydonlar**

| Holat | Soni | Nega normal |
|---|---|---|
| `last_activity_at` `lifecycle_since` dan 1 daqiqagacha oldin | 527 order | Bitta jarayonda ikki maydon ketma-ket yoziladi — soniyaning bir qismiga teng farq. Shuning uchun `activity_before_lifecycle_start` da 1 daqiqa tolerans |
| `payment_status = unpaid`, lekin `READY_TO_SHIP` | 8 (masalan BO1794401263UW) | Naqd to'lov (COD): xaridor tovarni olganda to'laydi |
| `item_count = 0`, `order_code` va `payment_status` bo'sh | 638 dan 599 | Stend bu maydonlarni ko'p orderlarda to'ldirmaydi: 0 / bo'sh = "noma'lum", "nol" emas. Order kodi o'rniga `group_no` ishlatiladi; `order_item_count_mismatch` faqat `item_count > 0` ni tekshiradi |

---

## API va hisobot

| Endpoint | Ma'nosi |
|---|---|
| `POST /sync` | sync'ni ishga tushiradi, tugaguncha kutadi va natijani qaytaradi (`inserted/updated/unchanged`, har obyekt bo'yicha). 409 — sync allaqachon ketyapti; 502 — stend xatosi (qisman natija bilan) |
| `GET /sync/status` | oxirgi sync (`last_run`), oxirgi muvaffaqiyatli sync (`last_success`) va hozir sync ketyaptimi. Stend o'chiq bo'lsa faqat sync ishlamaydi — anomaliyalar, timeline va hisobot bazadagi ma'lumot bilan ishlayveradi |
| `GET /anomalies?rule=&severity=` | anomaliyalar; ikkala filtr ixtiyoriy. Noma'lum `rule` yoki `severity` (`low`/`medium`/`high` emas) — 400. Mavjud rule + boshqa severity — 200, bo'sh ro'yxat |
| `GET /orders/{code}/timeline` | order, qutilari, hodisalar vaqt bo'yicha va shu orderning anomaliyalari. `code` — `order_code`, `group_no` yoki `id`; topilmasa 404 |
| `GET /report` | hisobot sahifasi: har qoida uchun noyob orderlar soni, 3 ta misol (3 ta turli order) va izoh |
| `GET /report.csv` | o'sha hisobot CSV'da, har qoida bitta qator: `rule,severity,orders_count,example_1,example_2,example_3,comment` |
| `GET /anomalies.csv` | to'liq ro'yxat: `rule,severity,order_code,reason` |

- `/anomalies` javobida har qoida uchun `total` — **noyob** orderlar soni (bitta orderda ikki handover bo'lsa ham bir marta),
  `total_orders` — ko'rsatilgan qoidalar bo'yicha noyob orderlar (bitta order ikki qoidaga tushsa ham bir marta).
- Hisobot, CSV, `/anomalies` va timeline bitta manbadan (`service/anomaly.go` `GetList`) oladi — sonlar har joyda bir xil.
- Hech bir endpoint stendga murojaat qilmaydi, faqat `POST /sync` (u ham faqat login + GET). Bazaga faqat sync yozadi.

Misol:
```bash
curl -s 'localhost:8080/anomalies?rule=dispatched_package_on_shelf'
# {"total_orders":1,"items":[{"rule":"dispatched_package_on_shelf","severity":"medium","total":1,
#   "items":[{"order_code":"HC1794401226GR","reason":"status=DISPATCHED, javonda qolgan qutilar: PKG-10192 0"}], ...}]}
curl -s 'localhost:8080/anomalies?rule=nope'     # 400 {"error":"noma'lum rule \"nope\"; mavjudlari: ..."}
curl -s 'localhost:8080/orders/NOPE/timeline'    # 404 {"error":"order topilmadi"}
```

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
| Timeline | hodisalar tartibi, bo'sh vaqtlar, 404, handler javoblari, faqat shu orderning anomaliyalari |
| Qoidalar (9 ta) | har qoida real Postgres'da: anomaliya qatorlari va normal qatorlar birga yoziladi, qoida **faqat** anomaliyalarni qaytarishi tekshiriladi (sabab matni bilan). Normal qatorlar ichida tahlilda topilgan yolg'on signallar bor: qisman qaytish, RETURN orderda STAGING, `item_count = 0`, 1 daqiqagacha farq, yo'ldagi handover kamomadi, ombordan chiqqan quti noma'lum javonda. Bo'sh bazada hamma qoida `[]` qaytaradi |
| Anomaliyalar API va hisobot | noyob orderlar soni, 3 ta turli misol, noma'lum rule/severity → 400, mavjud rule + boshqa severity → bo'sh, `/report.csv` qatorlari |


---

## AI bilan ishlash

Loyihada AI (Claude Code) yordamida ishladim.

**Men nima qildim.** Asosiy anomaliya yo'nalishlarini o'zim belgiladim:
- order statuslari bir-biriga mos emasligi;
- order va uning qutilari bir-biriga mos emasligi;
- order (qutisi) noma'lum, ya'ni aslida mavjud bo'lmagan javonda turgani.

**AI nima taklif qildi.** Shu yo'nalishlarga qarab AI vaqtga oid qoidalarni taklif qildi: yaratilishidan oldin
yo'lga chiqish yoki qabul qilish, bosqich boshlanishidan oldingi faollik, yetkazib berish holatida ko'p kun qolib
ketish. Ulardan bir qismini qabul qildim (`handover_timestamps_out_of_order`, `activity_before_lifecycle_start`).
"Ko'p kun yo'lda qolib ketish"ni esa qoida qilmadim: ma'lumotda ziddiyat yo'q, faqat taxminiy chegara — u
[Shubhali](#2-shubhali-otkazib-yuborildi) bo'limida.

**AI qayerda xato qildi.** AI biznes mantiqni repo (storage) qatlamiga yozmoqchi bo'ldi. Bu to'g'ri kelmasligini
ko'rib, `service` qatlamini qo'shdim: endi handler → service → repo; repo faqat bazadan o'qiydi/yozadi,
mantiq (filtrlash, hisoblash, timeline yig'ish) service'da.
