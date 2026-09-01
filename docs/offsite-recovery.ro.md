# Off-site și recuperare

Backupurile locale te protejează de un container pierdut sau o actualizare defectuoasă. Replicarea off-site și un kit de recuperare testat te protejează de întreaga stație, de ransomware sau de un incendiu. Această pagină acoperă replicarea off-site, transformarea acelei copii în una rezistentă la manipulare, dovedirea că poți restaura și recuperarea când BombVault însuși a dispărut.

## Replicare off-site

Păstrează backupul local rapid și adaugă una sau mai multe replici off-site. Setează un depozit per domeniu în fila **Setări, Off-site**. BombVault replică acolo instantaneele noi cu `restic copy` pe bază de best-effort, astfel încât o problemă off-site nu eșuează niciodată backupul local. Depozitul local rămâne principal.

- **Mai multe ținte off-site per domeniu.** Fiecare domeniu (containere, VM-uri, flash, config și seturi de fișiere) poate replica către mai multe destinații off-site simultan, nu doar una, așa că poți păstra, de exemplu, un rest-server pe stația unui prieten și un bucket S3 în paralel. Adaugă ținte suplimentare în Setări, Off-site, fiecare cu propriul depozit, clasă de stocare S3, indicator append-only, retenție și buget de creștere. O configurare off-site unică existentă este preluată neatinsă ca prima țintă, iar fiecare țintă a unui domeniu replică conform programării off-site a acelui domeniu.
- **Programare off-site per domeniu** (editată alături de fiecare altă programare în Setări, Programări): las-o goală pentru a replica după fiecare backup local, sau setează o cadență (de exemplu `weekly Sun 03:00`) pentru a trimite off-site mai rar decât faci backup local. Un buton **Replicate now** acoperă rulările la cerere.
- **Retenția off-site** se află în Setări, Off-site astfel încât să poți păstra copiile off-site mai mult timp ca arhivă. Las-o politica toată zero pentru a nu tăia niciodată automat instantaneele off-site.
- **Limitele de lățime de bandă** (Setări, Off-site) limitează rata de upload/download restic astfel încât replicarea să nu satureze WAN-ul tău.
- Un **indicator de replicare** arată care domeniu se replică în timp ce rulează (pe pagina sa și pe panoul principal). Este un indicator activ, nu o bară de procente, deoarece `restic copy` nu expune niciun progres citibil de mașină.

!!! note "Restaurează direct din off-site"
    Fiecare browser de backup are un comutator **Local / Off-site**, așa că dacă un depozit local este pierdut sau corupt poți lista și restaura direct din replica off-site. Ștergerea este per sursă: eliminarea unui backup afectează doar copia pe care o vizualizezi.

## Off-site imuabil (append-only)

Marchează un depozit off-site ca append-only astfel încât ransomware-ul, sau o gazdă compromisă, să nu poată șterge sau rescrie backupurile tale. Partea îndepărtată (un `restic/rest-server` rulând în mod `--append-only`) **o impune**. BombVault doar **o verifică** și nu arată niciodată verde doar pe baza unei afirmații de configurare.

Asistentul de **configurare off-site ghidată** te conduce de la alegerea backend-ului (rest-server / rclone / S3) printr-un fragment de deploy rest-server gata de lipit, un test de conexiune, comutatorul de imuabilitate (care rulează imediat testul de manipulare) și o strategie de retenție, astfel încât off-site-ul append-only este accesibil fără editarea manuală a configurațiilor.

!!! warning "Depozitele imuabile nu sunt niciodată curățate de pe această stație"
    Un off-site imuabil nu curăță niciodată în mod deliberat instantaneele vechi. Setează o **alarmă de buget de creștere** pentru el astfel încât să fii alertat înainte ca dimensiunea depozitului să scape de sub control.

## Test de manipulare

BombVault dovedește periodic garanția append-only încercând efectiv o ștergere împotriva depozitului off-site, îndreptată către un obiect inexistent:

- **Refuzată** înseamnă protejat.
- **Acceptată** înseamnă neprotejat.
- Un rezultat **neconcludent** (server inaccesibil, eroare de autentificare) nu răstoarnă niciodată verdictul stocat.

O trecere reală de la protejat la neprotejat declanșează o singură alertă.

## Exerciții DR

BombVault oferă două niveluri de dovadă că backupurile tale sunt efectiv restaurabile, nu doar prezente.

- **Exerciții de verificare a restaurării (local).** BombVault rulează periodic `restic check --read-data-subset` (mărginit, niciodată o restaurare completă care umple discul) și arată o insignă *ultima dată verificat ca restaurabil* per domeniu. Cadența se află în Setări, Programări; insigna în Setări, Integritate.
- **Exerciții DR (off-site).** BombVault restaurează o țintă reală din depozitul off-site într-un sandbox de unică folosință, o verifică fișier cu fișier și octet cu octet, apoi curăță. Aceasta dovedește că poți recupera din off-site, nu doar că depozitul răspunde.

**Fișa de evaluare a protecției împotriva ransomware** de pe panoul principal rezumă acestea într-o postură verde / galben / roșu per domeniu, cu o listă de verificare marcată cu vârsta (off-site configurat, append-only verificat, replicare curentă, exercițiu de restaurare trecut, criptare activată, strategie de curățare setată). Fiecare rând roșu are link direct către remediu, iar cardul devine verde doar pe fapte verificate.

## Panou de recepție (partea de recepție)

Tot ce este mai sus este partea de *trimitere*. Pe stația care **primește** copii off-site imuabile de la un alt BombVault, panoul de recepție îți oferă monitorizare independentă, doar în citire, a acelor depozite pe hardware-ul care primește, astfel încât o eșuare tăcută la capătul îndepărtat să nu treacă neobservată.

Activează comutatorul **Receiver** în Setări pentru a dezvălui o filă **Receiver**. Este oprit implicit; activează-l doar pe o stație care primește efectiv backupuri off-site imuabile. Apoi înregistrează un depozit primit (doar în citire, deschis cu cheia instanței care trimite) pentru a obține:

- **Un inventar de instantanee grupat pe sursă**, astfel încât să poți vedea exact care containere, VM-uri și seturi de fișiere au sosit.
- **Ultima primire** per sursă, astfel încât să știi cât de proaspătă este fiecare.
- **Un `restic check` independent** rulat pe hardware-ul care primește, astfel încât integritatea este verificată acolo unde stau efectiv datele, nu doar pe expeditor.
- **Un comutator de tip dead-man:** o alertă când o sursă încetează să trimită într-o fereastră pe care o setezi.
- **Alerte de integritate:** o alertă când o verificare pe partea de recepție eșuează.

Receiver-ul este strict doar în citire. Nu scrie niciodată în depozitul primit, deci nu poate niciodată strica garanția append-only pe care se bazează expeditorul.

## Exemplu complet: două mașini Unraid, de la un capăt la altul

Mai sus sunt descrise piesele. Aici este o configurație completă cu valori reale, pentru că piesele se asamblează mai ușor după ce le-ai văzut asamblate o dată.

Două mașini: **TOWER** rulează containerele și trimite copiile, **VAULT** le primește și impune imutabilitatea. Înlocuiește cu propriile nume, adrese și căi de partajare.

**1. Pe VAULT, ridică serverul append-only.** În BombVault pe TOWER mergi la *Setări → În afara sediului → configurare ghidată*, alege **rest-server** și generează rețeta. Copiază fila **Șablon Unraid (XML)**, salveaz-o pe VAULT ca `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, apoi *Docker → Add Container* și alege **rest-server** din lista de șabloane. Înainte de pornire, scrie linia `htpasswd` afișată în `/mnt/user/appdata/rest-server/.htpasswd` pe VAULT. Parola de unică folosință este afișată o singură dată și nu este niciodată păstrată: copiaz-o acum.

    Lasă `--append-only` în câmpul OPTIONS. Acesta este tot rostul: fără el, VAULT redevine o partajare obișnuită.

**2. Pe TOWER, îndreaptă depozitul extern către el.** Adresa depozitului urmează modelul tipărit de rețetă:

    rest:http://VAULT:8000/bombvault-containers/containers

Primul segment al căii este utilizatorul htpasswd, al doilea este depozitul. Introdu utilizatorul și parola generate ca acreditări REST ale destinației și rulează **testul de conexiune**.

**3. Pe TOWER activează „Imutabil”.** Testul de alterare rulează imediat și trebuie să spună *protejat*. Ce înseamnă răspunsurile:

| Rezultat | Ce s-a întâmplat |
| --- | --- |
| **protejat** | VAULT a refuzat ștergerea. Este singura stare care trece. |
| **NU este protejat** | VAULT a acceptat o ștergere. Lipsește `--append-only` sau a fost scos. |
| **neconcludent** | Nici una, nici alta. De obicei adresa nu este cea folosită de restic însuși, sau acreditările s-au schimbat. Nu se înregistrează nimic și nu se declanșează nicio alertă. |

**4. Pe VAULT, urmărește ce sosește.** Activează *Setări → Receptor*, deschide fila **Receptor** și înregistrează depozitul doar pentru citire.

!!! warning "Locația este o cale **din interiorul** containerului, scrisă relativ la montarea gazdei"
    Introdu `user/appdata/rest-server/bombvault-containers/containers`, **nu** `/mnt/user/appdata/…`. BombVault rulează într-un container unde `/mnt` al gazdei este montat în altă parte; o cale absolută a gazdei nu există acolo. Dacă lipești una, BombVault îți spune acum ce cale relativă să folosești.

    **APP_KEY-ul expeditor** este cheia TOWER, nu a VAULT. O găsești pe TOWER la *Setări → Sistem*.

**5. Fă-l reciproc, dacă vrei.** Repetă aceiași cinci pași în sens invers: un rest-server pe TOWER care primește copia VAULT. Atunci fiecare mașină impune imutabilitatea pentru cealaltă, și niciuna nu poate șterge copiile celeilalte.

## Recuperare ghidată

O filă dedicată **Recuperare** conduce o instalare nouă sau reconstruită prin cazul de dezastru, într-un singur loc:

1. **Restaurează mai întâi propriile setări ale BombVault**, astfel încât căile de backup, țintele off-site și credențialele de care restul fluxului are nevoie să fie precompletate (aplicate printr-o auto-repornire peste socket-ul Docker, astfel încât baza de date de setări în execuție să nu fie niciodată suprascrisă sub un handle deschis).
2. **Verifică dacă BombVault poate citi backupurile tale** (capcana cheii de criptare, în față).
3. Îți permite să **îndrepți către depozitul tău existent** (local sau off-site).
4. **Descoperă** containerele, VM-urile și seturile de fișiere stocate în el.
5. **Le restaurează pe toate** (lăsate oprite, ca să le pornești deliberat), cu kitul tău de recuperare la un clic distanță.

!!! tip "Migrare planificată versus dezastru"
    Recuperarea ghidată restaurează propriile setări ale BombVault dintr-un backup. Pentru o mutare *planificată* pe o stație nouă, poți în schimb să-ți muți configurația direct cu cardul **Export și import setări** (un fișier JSON portabil). Vezi [Configurare](configuration.md#portable-settings-export-and-import).

### Restaurare dintr-un alt depozit BombVault

Un card separat în fila **Recuperare** deschide depozitul unei *alte* instanțe BombVault (o partajare montată sub `/mnt`, sau un URL la distanță) cu **`APP_KEY`-ul acelei instanțe**, într-o sesiune unică, doar în citire. Răsfoiește containerele, VM-urile și seturile de fișiere stocate acolo, alege un instantaneu și restaurează-l, iar obiectul restaurat devine un container, VM sau set de fișiere local normal. Nimic nu este scris vreodată în celălalt depozit, iar propriile tale setări de backup rămân neatinse (sesiunea trăiește în memorie și expiră singură). Mutarea unui container de pe serverul A pe serverul B nu mai înseamnă repointarea setărilor depozitului tău și revenirea lor ulterioară. Federarea live server-la-server este explicit în afara scopului; aceasta este o extragere deliberată de unică folosință.

## Kit de recuperare a cheii de criptare

Aceasta este piesa care face recuperarea în caz de dezastru posibilă chiar și când nu există niciun BombVault în execuție.

Un clic descarcă **cheia principală**, **parola restic derivată** și **locațiile și comenzile exacte ale depozitului**, astfel încât să poți restaura direct cu CLI-ul restic pe orice mașină. O amintire pe panoul principal insistă până când l-ai stocat.

!!! danger "Stochează kitul de recuperare în afara serverului"
    Kitul conține secretul care decriptează backupurile tale. Păstrează-l undeva în siguranță și separat de server (un manager de parole, o copie printată într-un seif). Dacă pierzi atât BombVault cât și `APP_KEY` fără niciun kit de recuperare, backupurile tale criptate nu pot fi recuperate.

Deoarece definițiile de recuperare se află **în interiorul** fiecărui depozit (`<repo>/def`, `<repo>/vm-def`), un folder de depozit copiat este complet autonom, așa că kitul plus depozitul este tot ce are nevoie o restaurare bare-metal.
