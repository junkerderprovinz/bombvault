# Off-site și recuperare

Backupurile locale te protejează de un container pierdut sau o actualizare defectuoasă. Replicarea off-site și un kit de recuperare testat te protejează de întreaga stație, de ransomware sau de un incendiu. Această pagină acoperă replicarea off-site, transformarea acelei copii în una rezistentă la manipulare, dovedirea că poți restaura și recuperarea când BombVault însuși a dispărut.

## Replicare off-site

Păstrează backupul local rapid și copiază-l în unul sau mai multe alte locuri. Alegi locurile în care este copiat un domeniu pe cardul **Domenii** din **Setări, Stocare**, câte un chip pentru fiecare loc (vezi [Locuri de stocare](storage-places.md#domains)). BombVault copiază acolo instantaneele noi cu `restic copy` pe bază de best-effort, astfel încât o copiere eșuată nu eșuează niciodată backupul local. Locul în care este stocat un domeniu nu trebuie să fie local; vezi [Un domeniu stocat într-un loc la distanță](#remote-primary-repositories).

- **Mai multe locuri de copiere per domeniu.** Un domeniu poate fi copiat în mai multe locuri simultan, de exemplu pe un rest-server aflat la un prieten acasă și într-un bucket B2. Retenția, clasa de stocare, append-only, limitele și bugetul de creștere țin de loc, așa că fiecare copie urmează regulile locului în care ajunge.
- **Programare de copiere per domeniu** (editată alături de fiecare altă programare în Setări, Programări): las-o goală pentru a copia după fiecare backup local, sau setează o cadență (de exemplu `weekly Sun 03:00`) pentru a copia mai rar decât faci backup. **Copiază acum** de pe rândul domeniului o rulează la cerere.
- **Retenție per loc.** Fiecare loc își păstrează propriile reguli, așa că un loc off-site poate păstra copiile mai mult timp ca arhivă. Un loc cu toate regulile la zero nu curăță niciodată nimic.
- **Limitele de lățime de bandă** per loc limitează rata de încărcare și descărcare restic, astfel încât copierea să nu satureze WAN-ul tău.
- Un **indicator de replicare** arată care domeniu se copiază în timp ce rulează (pe pagina sa și pe panoul principal). Este un indicator activ, nu o bară de procente, deoarece `restic copy` nu expune niciun progres citibil de mașină.

!!! note "Restaurează din orice loc"
    Fiecare container, VM, set de fișiere, flash-ul și configurația aplicației își listează backupurile ca o singură cronologie de-a lungul tuturor locurilor unde stă o copie. O copie de siguranță trimisă în B2 apare o singură dată, marcată cu fiecare loc care o deține. O restaurare folosește primul loc pe care îl poate atinge, începând cu depozitul în care este scris elementul, și poți alege alt loc pentru fiecare rând. Locurile off-site sunt citite doar când le deschizi. Ștergerea într-un loc verifică mai întâi celelalte și spune dacă a fost ultima copie.

## Amplasare per element {#placement}

Fiecare card de container, VM și set de fișiere are un rând **Amplasare** cu trei segmente:

- **Local** scrie elementul în depozitul arătat la **Stocat pe** și nu îl copiază nicăieri. Folosește-l pentru date care au deja o a doua copie, de exemplu o partajare care trăiește pe un NAS.
- **Local + extern** îl scrie și acolo și îl copiază la țintele bifate la **Copiază în**, câte un chip pentru fiecare țintă off-site a domeniului. Debifează un chip și acea țintă nu mai primește nimic nou de la acest element.
- **Doar extern** scrie elementul direct în locul de la **Trimite către**, orice loc în afară de locul de stocare al domeniului. Dacă domeniul este deja copiat în acel loc, elementul primește un depozit direct alături de copii; altfel BombVault creează acolo un depozit pentru domeniu.

Locația este fixată de la prima copie de siguranță a elementului, pentru că BombVault nu mută niciodată copiile între depozite. Copiile se pot schimba oricând. O țintă care nu mai primește un element păstrează copiile pe care le are și le taie la propria retenție la următoarea rulare off-site a domeniului; **Șterge în B2** de pe card le elimină imediat. Când unele dintre acele copii nu există nicăieri altundeva, confirmarea le listează după dată și cere numele elementului. Din țintele append-only nu se poate șterge.

Sub rând, cardul spune unde ajunge elementul și ce se află de fapt acolo: câte locații îl dețin, când a fost văzută ultima dată fiecare țintă și dacă este îndeplinită regula 3-2-1. O locație este serverul cu datele originale și fiecare loc aflat într-o altă locație (vezi [În afara sediului](#off-the-premises-mark)). BombVault verifică copiile și locațiile; nu verifică partea de „două medii” a regulii 3-2-1.

### Valori implicite per domeniu

Cardul **Domenii** din Setări, Stocare are câte un rând pentru fiecare domeniu. **Copiat în** se aplică imediat fiecărui element fără alegere proprie, și folderelor de proiect ale stack-urilor Compose. Odată ce un domeniu are copii de siguranță, **Stocat în** se aplică unui element nou la prima lui copie de siguranță, iar schimbarea lui nu mută nicio copie de siguranță. Înainte de salvare, rândul numește fiecare loc care câștigă sau pierde elemente și câte instantanee înseamnă asta, iar întrebarea conține comutatorul **Aplică elementelor fără copii de siguranță**, care aplică noua valoare implicită și fiecărui element care nu are încă o copie de siguranță. **Excepții** listează elementele cu alegere proprie.

Bifarea unui loc nou la **Copiat în** îl face să primească orice element care nu este setat pe Local. Confirmarea spune câte elemente și, unde se știe, cât istoric înseamnă asta.

### Depozite directe

Alegerea sub Doar extern a unui loc în care domeniul este deja copiat întreabă o dată, apoi creează un depozit direct alături de copii, de exemplu `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, și îndreaptă elementul spre el. Pentru o țintă de copiere fără loc, alegerea deschide un dialog cu o adresă sugerată și un test de conexiune care nu creează nimic, iar **Creează și folosește** creează depozitul. Un depozit direct preia cheia, clasa de stocare, limitele, setarea append-only și retenția locului, și se schimbă odată cu ele. Când o cheie nouă a locului nu îl poate deschide, depozitul direct păstrează cheia pe care o are, iar salvarea spune asta. Instantaneele lui poartă eticheta `bv:direct`, iar fiecare altă trecere de retenție le păstrează, așa că un depozit direct care și-a pierdut legătura cu locul lui nu îmbătrânește niciodată după regulile locale. O cheie B2 limitată la un singur dosar trebuie să acopere adresa locului, nu doar dosarul domeniului, altfel dosarul de alături rămâne inaccesibil.

### În afara sediului {#off-the-premises-mark}

O copie contează ca locație separată doar când locul ei se află într-o altă locație. Un loc în cloud contează întotdeauna, iar un folder pe acest Unraid niciodată; pentru un NAS, un rest-server sau un server SFTP, răspunde la **Unde se află dispozitivul?** în detaliile locului cu **Aici, în casă** sau **Într-o altă locație**. Răspunsul servește doar la numărarea locațiilor și la regula 3-2-1 de pe carduri și de pe panoul principal. Nu schimbă nicio copie.

### După o reconstrucție

Alegerile de copiere trăiesc în propriile setări ale BombVault. După o reconstrucție prin Descoperă fără un `/config` restaurat, ele dispar, iar copierea a tot ar trimite din nou în B2 elementele pe care le lăsaseși deoparte. Replicarea off-site a fiecărui domeniu reconstruit se suspendă de aceea. Panoul principal arată asta în galben, iar rândul domeniului de pe cardul Domenii oferă **Confirmă valoarea implicită** cu o previzualizare a ceea ce copiază următoarea rulare și numele din backupuri care nu au o intrare, pe care le poți lăsa deoparte acolo. Doar confirmarea încheie suspendarea; importarea unui fișier de setări readuce regulile și valorile implicite, dar nu o încheie.

## Un domeniu stocat într-un loc la distanță {#remote-primary-repositories}

Un domeniu nu trebuie să fie stocat local. Cât timp calea lui de backup nu conține copii de siguranță, alege un loc la distanță la **Stocat în** pe cardul Domenii, iar domeniul salvează direct acolo, fără copie locală și fără pas de copiere. Depozitul la distanță este atunci singura copie, dacă domeniul nu este copiat și într-un alt loc. Fiecare loc la distanță vine cu aceleași măsuri de siguranță:

- **Un test de conexiune** înainte să se scrie ceva.
- **Limite de lățime de bandă** pentru backupul însuși, aceleași opțiuni `--limit-upload` și `--limit-download` pe care le folosește o copiere.
- **Protecție append-only**, verificată cu același test activ de manipulare. Cu ea pornită, BombVault nu curăță niciodată depozitul, pentru că acreditările de pe această mașină nu trebuie să poată șterge singura copie a backupului.
- **Un buget de creștere**, luat din aceeași tendință a dimensiunii pe care o urmărește cardul Stocare.

Un domeniu stocat într-un loc la distanță este sursa copiilor lui, la fel ca unul local; vezi [Copii între locuri cu date de acces diferite](storage-places.md#different-credentials).

!!! note "Acreditările țin de loc"
    Un loc la distanță își păstrează propriile acreditări. Un loc configurat cu acreditările cloud comune le folosește în continuare, până când accesul lui este schimbat în detaliile sale.

### SMB și WebDAV fără montare pe gazdă {#smb-webdav}

Formularul rclone din fereastra **Adaugă loc** are un formular pentru o partajare Windows sau Samba și pentru un server WebDAV (Nextcloud, ownCloud, SharePoint sau oricare altul). Completează un nume scurt, gazda și partajarea (SMB) sau URL-ul și tipul serverului (WebDAV), utilizatorul și parola, iar BombVault scrie secțiunea rclone pentru tine. rclone ascunde singur parola înainte să fie stocată; adăugarea unei destinații cu un nume care există deja înlocuiește acea secțiune în loc să adauge una a doua.

Noul remote apare apoi în lista de remote-uri a formularului, unde îl alegi pentru loc. Partajarea este primul segment al căii, nu face parte din nume.

Aceasta e o cale mai bună decât montarea partajării pe Unraid: restic nu recomandă păstrarea unui depozit pe o partajare CIFS montată, iar aici nu se montează nimic. NFS nu apare în formular pentru că nici restic, nici rclone nu au un backend NFS; pentru NFS, montează exportul pe gazdă și adaugă-l ca loc cu **Altă partajare**.

## Off-site imuabil (append-only)

Marchează un depozit off-site ca append-only astfel încât ransomware-ul, sau o gazdă compromisă, să nu poată șterge sau rescrie backupurile tale. Partea îndepărtată (un `restic/rest-server` rulând în mod `--append-only`) **o impune**. BombVault doar **o verifică** și nu arată niciodată verde doar pe baza unei afirmații de configurare.

Fereastra **Adaugă loc** conține o rețetă gata de lipit pentru un rest-server în mod append-only, cu un utilizator pentru acest BombVault. La un loc rest-server cu **Append-only** pornit, **Testează append-only** din detaliile locului rulează testul de manipulare pe fiecare cale de domeniu, fiecare copie pornită și fiecare depozit din acel loc și dă un singur răspuns pentru loc, astfel încât off-site-ul append-only este accesibil fără editarea manuală a configurațiilor.

!!! note "O ștergere reușită sub `/locks/` este așteptată"
    Append-only nu înseamnă că nu se mai poate șterge nimic. restic trebuie să își creeze și să își elibereze propriile blocaje, așa că `/locks/` rămâne intenționat inscriptibil și șterjibil. Instantaneele și datele din spatele lor, adică exact ținta unui ransomware, nu pot fi eliminate. Dacă testezi singur partea de la distanță, o ștergere reușită sub `/locks/` este comportament corect și nu o breșă.

!!! warning "Depozitele imuabile nu sunt niciodată curățate de pe această stație"
    Un off-site imuabil nu curăță niciodată în mod deliberat instantaneele vechi. Setează o **alarmă de buget de creștere** pentru el astfel încât să fii alertat înainte ca dimensiunea depozitului să scape de sub control.

## Test de manipulare

BombVault dovedește periodic garanția append-only încercând efectiv o ștergere împotriva depozitului off-site, îndreptată către un obiect inexistent:

- **Refuzată** înseamnă protejat.
- **Acceptată** înseamnă neprotejat.
- Un rezultat **neconcludent** (server inaccesibil, eroare de autentificare) nu răstoarnă niciodată verdictul stocat.

O trecere reală de la protejat la neprotejat declanșează o singură alertă.

La un loc, **Testează append-only** verifică fiecare cale de domeniu, fiecare copie pornită și fiecare depozit de acolo cu propriile credențiale și adună verdictele într-un singur răspuns: dacă un singur depozit acceptă o ștergere, întregul loc primește *ștergeri acceptate*.

## Exerciții DR

BombVault oferă două niveluri de dovadă că backupurile tale sunt efectiv restaurabile, nu doar prezente.

- **Exerciții de verificare a restaurării (local).** BombVault rulează periodic `restic check --read-data-subset` (mărginit, niciodată o restaurare completă care umple discul) și arată o insignă *ultima dată verificat ca restaurabil* per domeniu. Cadența se află în Setări, Programări; insigna în Setări, Integritate.
- **Exerciții DR (off-site).** BombVault restaurează o țintă reală din depozitul off-site într-un sandbox de unică folosință, o verifică fișier cu fișier și octet cu octet, apoi curăță. Aceasta dovedește că poți recupera din off-site, nu doar că depozitul răspunde. Exercițiile rulează doar pe locuri dintr-o altă locație, pentru că o copie din aceeași casă nu dovedește nimic despre pierderea casei. Un domeniu copiat în mai multe astfel de locuri este exersat pe câte unul la fiecare rulare programată, pe rând, iar panoul principal arată locul ultimului exercițiu.

**Fișa de evaluare a protecției împotriva ransomware** de pe panoul principal rezumă acestea într-o postură verde / galben / roșu per domeniu, cu o listă de verificare marcată cu vârsta (off-site configurat, append-only verificat, replicare curentă, exercițiu de restaurare trecut, criptare activată, strategie de curățare setată). Fiecare rând roșu are link direct către remediu, iar cardul devine verde doar pe fapte verificate.

## Panou de recepție (partea de recepție)

![Partea care primește, urmărită doar în citire, cu o verificare de integritate rulată pe această mașină.](assets/screenshots/receiver.png)

*Partea care primește, urmărită doar în citire, cu o verificare de integritate rulată pe această mașină.*

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

**1. Pe VAULT, ridică serverul append-only.** În BombVault pe TOWER deschide *Setări → Stocare*, apasă **Adaugă loc**, alege **rest-server** și apasă **Arată rețeta**. Copiază blocul **Șablon Unraid**, salvează-l pe VAULT ca `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, apoi *Docker → Add Container* și alege **rest-server** din lista de șabloane. Înainte de pornire, scrie linia `htpasswd` afișată în `/mnt/user/appdata/rest-server/.htpasswd` pe VAULT. Parola este afișată o singură dată și nu este niciodată păstrată; rețeta a pus-o deja, împreună cu utilizatorul, în formularul de pe TOWER, așa că lasă fereastra deschisă. Linia `htpasswd` poartă aceeași parolă, deja criptată cu bcrypt pentru tine, așa că nu trebuie să criptezi nimic tu.

    Lasă `--append-only` în câmpul OPTIONS. Fără el, VAULT redevine doar o partajare obișnuită.

**2. Pe TOWER, adaugă locul.** Introdu adresa lui VAULT, `http://VAULT:8000`, lângă utilizatorul și parola completate de rețetă, apoi apasă **Testează conexiunea**. BombVault construiește adresa din ele:

    rest:http://VAULT:8000/tower

Primul segment al căii este utilizatorul htpasswd, aici `tower`, iar fiecare domeniu își primește folderul sub el, de exemplu `rest:http://VAULT:8000/tower/container`. Răspunde la **Unde se află dispozitivul?** cu **Într-o altă locație**, apasă **Adaugă** și bifează locul la **Copiat în** pentru domeniile care trebuie să ajungă acolo.

**3. Pe TOWER, pornește Append-only** la **Protecție** în detaliile locului, apoi apasă **Testează append-only**. Testul verifică fiecare cale de domeniu, fiecare copie și fiecare depozit din acel loc și dă un singur răspuns pentru loc, care trebuie să fie *ștergeri refuzate*. Ce înseamnă răspunsurile:

| Rezultat | Ce s-a întâmplat |
| --- | --- |
| **ștergeri refuzate** | VAULT a refuzat ștergerea. Este singura stare care trece. |
| **ștergeri acceptate** | VAULT a acceptat o ștergere. Lipsește `--append-only` sau a fost scos. |
| un mesaj în loc de rezultat | Testul nu a putut rula. De obicei adresa nu este cea folosită de restic însuși, sau acreditările s-au schimbat. Nu se înregistrează nimic și nu se declanșează nicio alertă. |

**4. Pe VAULT, urmărește ce sosește.** Activează *Setări → Receptor*, deschide fila **Receptor** și înregistrează depozitul doar pentru citire.

!!! warning "Locația este o cale **din interiorul** containerului, scrisă relativ la montarea gazdei"
    Introdu `user/appdata/rest-server/tower/container`, **nu** `/mnt/user/appdata/…`. BombVault rulează într-un container unde `/mnt` al gazdei este montat în altă parte; o cale absolută a gazdei nu există acolo. Dacă lipești una, BombVault îți spune ce cale relativă să folosești.

    **APP_KEY-ul expeditor** este cheia TOWER, nu a VAULT. O găsești pe TOWER la *Setări → Sistem*.

**5. Fă-l reciproc, dacă vrei.** Repetă aceiași cinci pași în sens invers: un rest-server pe TOWER care primește copia VAULT. Atunci fiecare mașină impune imutabilitatea pentru cealaltă, și niciuna nu poate șterge copiile celeilalte.

## Recuperare ghidată

O filă dedicată **Recuperare** conduce o instalare nouă sau reconstruită prin cazul de dezastru, într-un singur loc:

1. **Verifică dacă BombVault poate citi backupurile tale** (capcana cheii de criptare, în față).
2. **Restaurează propriile setări ale BombVault**, astfel încât căile de backup, țintele off-site și credențialele de care restul fluxului are nevoie să fie precompletate. Backupul de setări este citit din locul pe care rândul Auto-backup îl numește la **Stocat în** sau din copia Auto-backup de la **Copiat în**, iar pasul arată acel loc cu adresa lui; ca să citești din alt loc, schimbă mai întâi rândul Auto-backup în pasul 3. Restaurarea este aplicată printr-o auto-repornire peste socket-ul Docker, astfel încât baza de date de setări în execuție să nu fie niciodată suprascrisă sub un handle deschis.
3. **Atașează backupurile tale existente** prin rândurile cardului Domenii: pe rândul fiecărui domeniu alegi la **Stocat în** locul în care se află backupurile lui și la **Copiat în** locurile care îi păstrează copiile. Un loc pe care încă nu îl oferă niciun rând, cum ar fi un share, un server sau un bucket în cloud, îl conectezi cu **Adaugă loc**, aceeași fereastră ca în Setări, Stocare. **Conectează și previzualizează** verifică apoi dacă backupurile pot fi citite.
4. **Descoperă** containerele, VM-urile, seturile de fișiere și seturile de date ZFS stocate în el.
5. **Restaurează containerele și VM-urile dintr-odată** (lăsate oprite, ca să le pornești deliberat) și listează seturile de fișiere și elementele ZFS de restaurat unul câte unul; elementele ZFS revin dezactivate. Kitul tău de recuperare e la un clic distanță.

!!! note "Copiile off-site așteaptă după o reconstrucție"
    Când pasul 4 reconstruiește intrări fără setările vechi, replicarea off-site a acelor domenii se suspendă până când amplasarea implicită este confirmată. Vezi [Amplasare per element](#placement).

!!! tip "Migrare planificată versus dezastru"
    Recuperarea ghidată restaurează propriile setări ale BombVault dintr-un backup. Pentru o mutare *planificată* pe o stație nouă, poți în schimb să-ți muți configurația direct cu cardul **Export și import setări** (un fișier JSON portabil). Vezi [Configurare](configuration.md#portable-settings-export-and-import).

### Restaurare dintr-un alt depozit BombVault

Un card separat în fila **Recuperare** deschide depozitul unei *alte* instanțe BombVault (o partajare montată sub `/mnt`, sau un URL la distanță) cu **`APP_KEY`-ul acelei instanțe**, într-o sesiune unică, doar în citire. Răsfoiește containerele, VM-urile și seturile de fișiere stocate acolo, alege un instantaneu și restaurează-l, iar obiectul restaurat devine un container, VM sau set de fișiere local normal. Nimic nu este scris vreodată în celălalt depozit, iar propriile tale setări de backup rămân neatinse (sesiunea trăiește în memorie și expiră singură). Mutarea unui container de pe serverul A pe serverul B nu mai înseamnă repointarea setărilor depozitului tău și revenirea lor ulterioară. Federarea live server-la-server este explicit în afara scopului; aceasta este o extragere deliberată de unică folosință.

## Kit de recuperare a cheii de criptare

Aceasta este piesa care face recuperarea în caz de dezastru posibilă chiar și când nu există niciun BombVault în execuție.

Un clic descarcă **cheia principală**, **parola restic derivată** și **locațiile și comenzile exacte ale depozitului**, astfel încât să poți restaura direct cu CLI-ul restic pe orice mașină. O amintire pe panoul principal insistă până când l-ai stocat.

!!! danger "Stochează kitul de recuperare în afara serverului"
    Kitul conține secretul care decriptează backupurile tale. Păstrează-l undeva în siguranță și separat de server (un manager de parole, o copie printată într-un seif). Dacă pierzi atât BombVault cât și `APP_KEY` fără niciun kit de recuperare, backupurile tale criptate nu pot fi recuperate.

!!! warning "Cel mai nou snapshot nu este întotdeauna cel de restaurat"
    Începând cu restic 0.17, `restic snapshots` arată dimensiunea fiecărui snapshot. După o pierdere de date, cel mai nou snapshot poate fi cel golit, așa că nu restaura un snapshot mult mai mic decât cele dinaintea lui. După un ransomware poate fi cel criptat, de dimensiune obișnuită. Dacă BombVault încă rulează, uită-te mai întâi pe pagina sa **Anomalii**: ea numește ultimul backup bun. O restaurare nu are nevoie de niciun fel de date despre anomalii din BombVault, iar pauza retenției doar păstrează mai multe snapshoturi.

### Sigilarea kitului

Dacă ai activat criptarea age pentru exporturile în clar (Setări), kitul este sigilat și el cu ea și se descarcă drept `bombvault-recovery-kit.md.age`. Este în format ASCII armor, nu binar, deci rămâne text simplu: lipirea lui într-un manager de parole sau tipărirea funcționează exact ca înainte, doar că fără cheia ta conținutul nu poate fi citit.

!!! warning "Nu păstra cheia age în kit"
    Ca să deschizi un kit sigilat ai nevoie de cheia ta age **privată**. Păstreaz-o într-un loc care nu depinde de kitul însuși, altfel vei avea două lucruri de recuperat în loc de unul. Sigilarea merită când kitul stă într-un loc pe care nu îl controlezi pe deplin (un manager de parole partajat, notițe în cloud, o copie tipărită într-un birou); un kit în propriul seif este deja protejat de seif.

    Cu criptarea activă și fără niciun destinatar utilizabil configurat, descărcarea este refuzată de la început. BombVault nu recurge niciodată la a preda cheia principală în clar.

### Când kitul nu e la îndemână

Parola nu este stocată nicăieri, se **calculează** din `APP_KEY`. Cu cheia și un shell o poți reproduce singur:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Este HMAC-SHA256 peste șirul fix `bombvault:restic-repo`, cu octeții bruți ai `APP_KEY` hexazecimal drept cheie, tipărit ca 64 de caractere hexazecimale mici. Aceeași valoare se află în kit, ca parolă restic derivată; secțiunea asta e pentru ziua în care kitul e în altă parte decât tine.

!!! warning "Pentru un depozit primit, folosește cheia instanței EXPEDITOARE"
    Un depozit ajuns aici prin replicare în afara sediului a fost creat de mașina care l-a trimis, cu `APP_KEY`-ul **ei**. Derivarea din cheia mașinii care primește dă o parolă pe care restic o refuză, ceea ce arată exact ca un depozit corupt fără să fie. Acesta e motivul obișnuit pentru care `restic check` pe un depozit primit cere parola iar și iar.

Deoarece definițiile de recuperare se află **în interiorul** fiecărui depozit (`<repo>/def`, `<repo>/vm-def`), un folder de depozit copiat este complet autonom, așa că kitul plus depozitul este tot ce are nevoie o restaurare bare-metal.

## Recuperarea unui dump de bază de date {#database-dumps}

Un dump de bază de date este un punct de restaurare de sine stătător în depozitul containerelor, cu eticheta `dbdump:<container>` și un singur fișier, `/dbdump/<container>.sql`. BombVault le listează, le descarcă și le importă la **Backupuri**; mai jos sunt aceiași pași doar cu restic, pentru ziua în care BombVault nu este la îndemână.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Etichetele `dbversion:` și `dbname:` de pe fiecare dump spun din ce versiune de server provine și ce baze conține. Un fișier complet se termină cu `-- PostgreSQL database cluster dump complete` sau `-- Dump completed`.

Importă-l într-un container de aceeași versiune sau una mai nouă (PostgreSQL), ori de aceeași versiune majoră (MySQL și MariaDB), pornit o dată cu folderul de date gol ca să se inițializeze. Gazda nu are nevoie de client de bază de date, containerul are unul:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Pentru o singură bază dintr-un dump complet, MySQL și MariaDB acceptă `--one-database <name>` în comanda clientului. Un dump PostgreSQL are câte o secțiune pentru fiecare bază, fiecare începând cu o linie `\connect <name>`: copiază secțiunea într-un fișier propriu și importă-l cu `-d <name>` după ce ai creat baza.

!!! warning "Un dump luat ca root aduce cu el utilizatorii serverului"
    Un dump complet MySQL sau MariaDB luat ca root conține baza de sistem `mysql`, așa că importul înlocuiește conturile serverului nou, inclusiv parola de root, cu cele din dump. Pe PostgreSQL, `role ... already exists` pentru utilizatorul creat de container este de așteptat și inofensiv.
