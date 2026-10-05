# Off-site și recuperare

!!! note "Copiile off-site așteaptă după o reconstrucție"
    Când pasul 4 reconstruiește intrări fără setările vechi, replicarea off-site a acelor domenii se suspendă până când amplasarea implicită este confirmată. Vezi [Amplasare per element](#placement).

Backupurile locale te protejează de un container pierdut sau o actualizare defectuoasă. Replicarea off-site și un kit de recuperare testat te protejează de întreaga stație, de ransomware sau de un incendiu. Această pagină acoperă replicarea off-site, transformarea acelei copii în una rezistentă la manipulare, dovedirea că poți restaura și recuperarea când BombVault însuși a dispărut.

## Replicare off-site

Păstrează backupul local rapid și adaugă una sau mai multe replici off-site. Setează un depozit per domeniu în pagina **Setări, Extern**. BombVault replică acolo instantaneele noi cu `restic copy` pe bază de best-effort, astfel încât o problemă off-site nu eșuează niciodată backupul local. În această formă depozitul local rămâne primar, iar depozitul off-site este o replică, dar depozitul primar al unui domeniu nu trebuie deloc să fie local; vezi [Depozite primare la distanță](#remote-primary-repositories) mai jos pentru backup direct pe S3, rest-server etc., în loc de replicare către ele.

- **Mai multe ținte off-site per domeniu.** Fiecare domeniu (containere, VM-uri, flash, config, seturi de fișiere și seturi de date ZFS) poate replica către mai multe destinații off-site simultan, nu doar una, așa că poți păstra, de exemplu, un rest-server pe stația unui prieten și un bucket S3 în paralel. Adaugă ținte suplimentare în Setări, Extern, fiecare cu propriul depozit, clasă de stocare S3, indicator append-only, retenție și buget de creștere. O configurare off-site unică existentă este preluată neatinsă ca prima țintă, iar fiecare țintă a unui domeniu replică conform programării off-site a acelui domeniu.
- **Programare off-site per domeniu** (editată alături de fiecare altă programare în Setări, Programări): las-o goală pentru a replica după fiecare backup local, sau setează o cadență (de exemplu `weekly Sun 03:00`) pentru a trimite off-site mai rar decât faci backup local. Un buton **Replică acum** acoperă rulările la cerere.
- **Retenția off-site** se află în Setări, Retenție astfel încât să poți păstra copiile off-site mai mult timp ca arhivă. Las-o politica toată zero pentru a nu tăia niciodată automat instantaneele off-site.
- **Limitele de lățime de bandă** (Setări, Extern) limitează rata de upload/download restic astfel încât replicarea să nu satureze WAN-ul tău.
- Un **indicator de replicare** arată care domeniu se replică în timp ce rulează (pe pagina sa și pe panoul principal). Este un indicator activ, nu o bară de procente, deoarece `restic copy` nu expune niciun progres citibil de mașină.

!!! note "Restaurează din orice loc"
    Fiecare container, VM, set de fișiere, flash-ul și configurația aplicației își listează backupurile ca o singură cronologie de-a lungul tuturor locurilor unde stă o copie. O copie de siguranță trimisă în B2 apare o singură dată, marcată cu fiecare loc care o deține. O restaurare folosește primul loc pe care îl poate atinge, începând cu depozitul în care este scris elementul, și poți alege alt loc pentru fiecare rând. Locurile off-site sunt citite doar când le deschizi. Ștergerea într-un loc verifică mai întâi celelalte și spune dacă a fost ultima copie.

## Destinații {#destinations}

Setări, Extern începe cu **Destinații**: locurile în care ajung copiile off-site, configurate o singură dată pentru toate domeniile. O destinație apare apoi ca buton în rândul **Amplasare** al fiecărui domeniu și element. Prima dată când este bifată pentru un domeniu, BombVault creează depozitul acelui domeniu într-un dosar sub ea, de exemplu `rclone:onedrive:BombVault/containers`.

**Adaugă destinație** deschide un asistent în cinci pași:

1. **Unde să meargă copiile de rezervă?** Fiecare serviciu este listat cu sigla lui, în patru grupuri: servicii de stocare cu bucket-uri S3 (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 și altele), propriul tău server S3 (Garage, SeaweedFS, RustFS, Ceph, JuiceFS, Versity S3 Gateway), propriul tău server și partajările tale (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, o cale montată) și stocare în cloud (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud și restul serviciilor pe care le acceptă rclone). Fiecare spune cât de potrivit este pentru copii de rezervă: unitățile cloud încetinesc la multe cereri, așa că prima copie și curățarea durează mai mult acolo.
2. **Autentifică-te la** serviciul ales. Câmpurile depind de serviciu: o cheie de acces pentru S3, un utilizator și o parolă pentru WebDAV și SMB, o parolă de aplicație acolo unde autentificarea în doi pași o blochează pe cea obișnuită, cheia SSH publică a BombVault pentru SFTP și Storage Box, sau un token pentru serviciile care cer autentificare prin browser. Pentru acestea, asistentul arată o comandă `rclone authorize` de rulat pe un calculator cu browser; tokenul pe care îl afișează se pune în câmp. **Testează conexiunea** verifică autentificarea înainte să se salveze ceva.
3. **Alege un dosar.** Asistentul listează dosarele de pe destinație, cu **Dosar nou** pentru a crea unul și cu spațiul liber acolo unde serviciul îl raportează. Un dosar gol este cea mai sigură variantă.
4. **Protecție împotriva ștergerii.** Asistentul spune deschis ce poate face serviciul. Un rest-server în modul append-only refuză ștergerea, iar testul de manipulare verifică asta. Un bucket S3 poate păstra versiuni vechi prin versionare și blocarea obiectelor, lucru pe care BombVault nu îl poate verifica încă. O unitate cloud nu poate refuza deloc ștergerea: cine ajunge pe server ajunge și la acea copie. Activează **Imuabil (append-only)** doar acolo unde partea îndepărtată refuză cu adevărat ștergerea; BombVault nu mai curăță atunci niciodată acolo.
5. **Pentru o urgență.** Kitul de recuperare listează fiecare destinație cu depozitul fiecărui domeniu de sub ea. Autentificarea revine odată cu backupul de setări al BombVault; pe o instalare nouă fără el, configurează din nou destinația în același loc.

Serviciile S3 rulează prin backendul S3 propriu al restic, ceea ce permite aplicarea unei clase de stocare și a blocării obiectelor. Orice alt serviciu rulează prin rclone livrat cu BombVault, iar remote-ul lui apare apoi în configurația rclone de la Setări, Acces cloud. Un export al setărilor conține destinațiile; cu acreditările incluse, conține și autentificarea lor.

Ținta unui domeniu creată dintr-o destinație preia numele, locația, acreditările, clasa de stocare și comutatorul imuabil ale destinației. Retenția, compresia și bugetul de creștere rămân per domeniu, iar locația ei nu se poate muta, pentru că acolo se află depozitul domeniului. **Adaugă o țintă doar pentru acest domeniu** de sub fiecare domeniu primește în continuare un URL de depozit scris de mână.

## Amplasare per element {#placement}

Fiecare card de container, VM și set de fișiere are un rând **Amplasare** de butoane: **Local** și câte un buton pentru fiecare țintă off-site a domeniului, urmate de destinațiile sub care domeniul nu are încă nicio țintă. Butoanele aprinse primesc copiile de rezervă ale elementului.

- Cu **Local** aprins, elementul este scris în depozitul arătat la **Stocat pe** și copiat la fiecare altă țintă aprinsă. Stinge o țintă și ea nu mai primește nimic nou de la acest element. Doar Local nu copiază nicăieri, ceea ce se potrivește datelor care au deja o a doua copie, de exemplu o partajare care trăiește pe un NAS.
- Cu **Local** stins, elementul este scris direct în depozitul direct al primei ținte aprinse și copiat de acolo la celelalte ținte aprinse. Prima dată, un dialog creează acel depozit direct.
- Un buton de destinație creează ținta domeniului sub acea destinație și o aprinde doar pentru acest element. Fiecare alt element începe fără copie acolo.
- Un buton rămâne mereu aprins, pentru că o copie de rezervă are nevoie de un loc unde să meargă. Pentru a lăsa ceva în afara copiilor de rezervă, exclude-l.

Locația este fixată de la prima copie de siguranță a elementului, pentru că BombVault nu mută niciodată copiile între depozite. Copiile se pot schimba oricând. O țintă care nu mai primește un element păstrează copiile pe care le are și le taie la propria retenție la următoarea rulare off-site a domeniului; **Șterge în B2** de pe card le elimină imediat. Când unele dintre acele copii nu există nicăieri altundeva, confirmarea le listează după dată și cere numele elementului. Din țintele append-only nu se poate șterge.

Sub rând, cardul spune unde ajunge elementul și ce se află de fapt acolo: câte locuri îl dețin, când a fost văzută ultima dată fiecare țintă și dacă este îndeplinită regula 3-2-1. Un loc este serverul cu datele originale, fiecare țintă off-site și fiecare depozit marcat **În afara sediului**. BombVault verifică copiile și locurile; nu verifică partea de „două medii” a regulii 3-2-1.

### Amplasări implicite

Setări, Stocare, **Amplasări implicite** are un rând per domeniu cu aceleași butoane. Copiile se aplică imediat fiecărui element fără alegere proprie, și folderelor de proiect ale stack-urilor Compose. Locația se aplică unui element nou la prima lui copie de siguranță; schimbarea ei nu mută nicio copie. Înainte de salvare, rândul numește fiecare țintă care câștigă sau pierde elemente și câte instantanee înseamnă asta. **Aplică elementelor fără copii de siguranță** readuce la valoarea implicită orice element care nu are încă o copie de siguranță.

O țintă off-site nouă primește orice element care nu este setat pe Local. Dialogul care o adaugă spune câte elemente și, unde se știe, cât istoric înseamnă asta, și oferă opțiunea de a lăsa deoparte elementele deja excluse din alte ținte.

### Depozite directe

Oprirea opțiunii Local pentru un element, astfel încât o țintă fără depozit direct să devină casa lui, deschide un dialog cu o locație sugerată lângă țintă, de exemplu `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, și un test de conexiune care nu creează nimic. **Creează și folosește** creează depozitul și îndreaptă elementul spre el. Un depozit direct preia cheia, clasa de stocare, limitele, setarea append-only și retenția țintei, și se schimbă odată cu ele; cardul Depozite îl arată doar în citire. Când o cheie nouă a țintei nu îl poate deschide, depozitul direct păstrează cheia pe care o are, iar salvarea spune asta. Un element aflat pe un depozit direct este copiat de acolo la celelalte ținte aprinse, niciodată la ținta căreia îi aparține depozitul. Instantaneele lui poartă eticheta `bv:direct`, iar fiecare altă trecere de retenție le păstrează, așa că un depozit direct care și-a pierdut legătura cu ținta lui nu îmbătrânește niciodată după regulile locale. B2 se accesează prin punctul său final S3, cu ID-ul cheii și cheia aplicației introduse ca acreditări S3; o cheie limitată la dosarul propriu al țintei nu poate ajunge la dosarul alăturat, așa că limitează în schimb cheia la dosarul de deasupra țintei.

### În afara sediului

Un depozit numit poate fi marcat **În afara sediului** pe cardul Depozite. Depozitele la distanță pornesc marcate; dezactivează asta pentru un rest-server din aceeași clădire. Marcajul contează doar pentru locuri și pentru regula 3-2-1 de pe carduri. Nu schimbă nicio copie.

### După o reconstrucție

Alegerile de copiere trăiesc în propriile setări ale BombVault. După o reconstrucție prin Descoperă copii de rezervă fără un `/config` restaurat, ele dispar, iar copierea a tot ar trimite din nou în B2 elementele pe care le lăsaseși deoparte. Replicarea off-site a fiecărui domeniu reconstruit se suspendă de aceea. Panoul principal arată asta în galben, iar Amplasările implicite oferă **Confirmă valoarea implicită** cu o previzualizare a ceea ce copiază următoarea rulare și numele din backupuri care nu au o intrare, pe care le poți lăsa deoparte acolo. Doar confirmarea încheie suspendarea; importarea unui fișier de setări readuce regulile și valorile implicite, dar nu o încheie.

## Depozite primare la distanță {#remote-primary-repositories}

Calea de copiere a unui domeniu (Setări, Stocare) nu se limitează la un dosar local: îndreapt-o direct către un depozit restic la distanță (`s3:...`, `rest:http://gazda:8000/depozit`, `sftp:utilizator@gazda:/depozit`, `rclone:remote:bucket/cale`) și BombVault salvează direct acolo, fără copie locală separată și fără pas de replicare. Este o formă cu adevărat diferită de replicarea în afara sediului de mai sus: acolo depozitul local este cel primar, iar cel din afara sediului este o arhivă a lui, pe cât posibil; aici depozitul la distanță **este** cel primar și este singura copie, atâta timp cât nu configurezi și o replicare în afara sediului (sau un al doilea depozit la distanță) pentru acel domeniu.

Fiecare dintre cele șase câmpuri de cale (Containere, VM-uri, Flash, Auto-backup, Foldere, Seturi de date ZFS) are chiar alături un comutator **Local / La distanță**:

- **Local** arată exploratorul de dosare obișnuit.
- **La distanță** îl schimbă cu un simplu câmp de URL, plus un buton care deschide același dialog de test al conexiunii și de acreditări folosit de destinațiile din afara sediului, configurat însă pentru acest depozit primar. De acolo obții:
    - **Un test de conexiune** pe calea reală, înainte să te bazezi pe ea.
    - **Limite de lățime de bandă** (încărcare și descărcare), ca o copie programată către un depozit primar la distanță să nu îți sature legătura WAN: aceleași opțiuni restic `--limit-upload` și `--limit-download` folosite de replicarea în afara sediului, aplicate acum copiei înseși.
    - **Protecție append-only (imutabilitate)**, verificată cu același test activ de alterare (o sondă DELETE reală către cealaltă parte) pe care îl primesc destinațiile din afara sediului. Cu ea pornită, BombVault refuză să curețe el însuși depozitul: cum în spate nu există o copie locală separată, acreditările de pe această mașină nu trebuie să poată șterge singura copie a datelor salvate.
    - **O alarmă de buget al creșterii**, luată din aceeași tendință a dimensiunii depozitului pe care fișa Stocare o urmărește deja.

Nimic din toate acestea nu este obligatoriu: o cale la distanță scrisă de mână, fără setări de siguranță salvate, salvează exact ca înainte (lățime de bandă nelimitată, se poate curăța, fără alarmă de buget). Dialogul de siguranță este acolo pentru când vrei aceleași protecții pe care le primește o copie din afara sediului, fără să fii nevoit să creezi o destinație în afara sediului doar pentru asta.

!!! note "Acreditările pentru cloud și REST sunt comune"
    Un depozit primar la distanță se autentifică cu aceleași acreditări S3/REST configurate la Setări, Acces cloud, Credențiale cloud partajate. Nu există un depozit separat de acreditări pentru depozitele primare.

### SMB și WebDAV fără montare pe gazdă {#smb-webdav}

Setări, Acces cloud, rclone are un formular pentru o partajare Windows sau Samba și pentru un server WebDAV (Nextcloud, ownCloud, SharePoint sau oricare altul). Completează un nume scurt, gazda și partajarea (SMB) sau URL-ul și tipul serverului (WebDAV), utilizatorul și parola, iar BombVault scrie secțiunea rclone pentru tine. rclone ascunde singur parola înainte să fie stocată; adăugarea unei destinații cu un nume care există deja înlocuiește acea secțiune în loc să adauge una a doua.

Formularul răspunde cu locația finală, de exemplu `rclone:nas:backups`. Pune-o într-o cale de backup sau într-o destinație off-site și adaugă un subdosar dacă vrei (`rclone:nas:backups/bombvault`). Partajarea este primul segment al căii, nu face parte din nume.

Este o cale mai bună decât montarea partajării pe Unraid: restic nu recomandă păstrarea unui depozit pe o partajare CIFS montată, iar aici nu se montează nimic. NFS nu este în formular pentru că nici restic, nici rclone nu au un backend NFS; pentru NFS, montează exportul pe gazdă și îndreaptă o cale de backup către el.

## Off-site imuabil (append-only)

Marchează un depozit off-site ca append-only astfel încât ransomware-ul, sau o gazdă compromisă, să nu poată șterge sau rescrie backupurile tale. Partea îndepărtată (un `restic/rest-server` rulând în mod `--append-only`) **o impune**. BombVault doar **o verifică** și nu arată niciodată verde doar pe baza unei afirmații de configurare.

Asistentul de **configurare off-site ghidată** te conduce de la alegerea backend-ului (rest-server / rclone / S3) printr-un fragment de deploy rest-server gata de lipit, un test de conexiune, comutatorul de imuabilitate (care rulează imediat testul de manipulare) și o strategie de retenție, astfel încât off-site-ul append-only este accesibil fără editarea manuală a configurațiilor.

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

## Exerciții DR

BombVault oferă două niveluri de dovadă că backupurile tale sunt efectiv restaurabile, nu doar prezente.

- **Exerciții de verificare a restaurării (local).** BombVault rulează periodic `restic check --read-data-subset` (mărginit, niciodată o restaurare completă care umple discul) și arată o insignă *Restaurare verificată* per domeniu. Cadența se află în Setări, Programări; insigna în Setări, Integritate.
- **Exerciții DR (off-site).** BombVault restaurează o țintă reală din depozitul off-site într-un sandbox de unică folosință, o verifică fișier cu fișier și octet cu octet, apoi curăță. Aceasta dovedește că poți recupera din off-site, nu doar că depozitul răspunde.

**Fișa de evaluare a protecției împotriva ransomware** de pe panoul principal rezumă acestea într-o postură verde / galben / roșu per domeniu, cu o listă de verificare marcată cu vârsta (off-site configurat, append-only verificat, replicare curentă, exercițiu de restaurare trecut, criptare activată, strategie de curățare setată). Fiecare rând roșu are link direct către remediu, iar cardul devine verde doar pe fapte verificate.

## Împerecherea instanțelor {#pairing}

Receptorii, sursele de preluare, pagina Instanțe și Mesh off-site vorbesc toate cu un alt BombVault. O fac ca membri ai unui singur grup de împerechere, iar o instanță se alătură grupului cu douăsprezece cuvinte.

Pe prima instanță, deschide **Setări → Împerechere** și apasă pe **Generează frază** în cardurile de împerechere. Apar douăsprezece cuvinte într-o fereastră cu un buton **Copiază**. Pe fiecare altă instanță, deschide același loc, apasă pe **Introdu frază** și lipește-le sau tastează-le, sau apasă pe **Lipește** în acea fereastră. Un cuvânt care nu se află pe listă este numit împreună cu poziția lui chiar în timp ce îl tastezi, iar ultimul cuvânt poartă o sumă de control, așa că un cuvânt scris greșit sau schimbat între ele este prins înainte să se formeze împerecherea. Generează fraza pe o singură instanță: două instanțe care generează amândouă câte o frază formează două grupuri separate. Dacă nu apare nimeni timp de un minut, fila oferă două căi de ieșire: afișează din nou cuvintele ca să le introduci acolo, sau introdu cuvintele celeilalte instanțe și alătură-te grupului ei într-un singur pas. Împerecherea funcționează și fără o parolă de autentificare, dar setează una: fără ea, oricine poate deschide această interfață web poate citi cuvintele și obține, prin grup, parola restic a fiecărei instanțe din el. Cardul de împerechere arată asta până când se setează o parolă. Cu o parolă, reafișarea frazei o cere. **Părăsește grupul** scoate o instanță din nou afară.

Oricine cunoaște cuvintele se poate alătura grupului, așa că tratează-le ca pe o parolă.

**Cum se găsesc membrii între ei.** Fiecare instanță își află propria adresă din rețea de la browserul tău chiar în momentul autentificării, arătată pe cardul de releu ca **Această instanță în rețeaua ta**; corecteaz-o acolo dacă în față se află un reverse proxy sau un port neobișnuit. În aceeași rețea, membrii anunță acea adresă prin multicast și vorbesc direct între ei, iar acolo unde multicast-ul nu poate traversa o rețea de containere, cum e rețeaua bridge implicită a Docker, o instanță își caută în schimb propriul subnet pentru a găsi celelalte, cu un apel semnat la care poate răspunde doar un membru al grupului, așa că împerecherea tot se încheie în câteva secunde, fără releu. Dacă nu apare nimic, **Nu o găsești?** de sub cardul de împerechere primește o adresă introdusă manual, pentru un alt subnet sau un port neobișnuit. Instanțele din rețele diferite trec printr-un releu, ales pe aceeași filă:

- **Relay de proiect** (implicit): `parleyport.halleluja.design`, același releu pe care îl folosește și KnightLoader. Nimic de configurat.
- **Relay propriu**: containerul [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) din Unraid Community Apps, sau una dintre instanțele tale care este deja accesibilă din exterior cu **Servește ca relay** activat. Acea instanță răspunde apoi la `/relay/connect` pe propria adresă, în spatele reverse proxy-ului și certificatului pe care le are deja, și lasă să intre doar grupul tău. Introdu adresa releului pe fiecare instanță care ar trebui să îl folosească.
- **Fără relay**: membrii se găsesc automat în aceeași rețea, și nicăieri altundeva.

**Ce vede releul.** Fiecare apel dintre membri este sigilat cu AES-256-GCM sub o cheie derivată din cele douăsprezece cuvinte, iar acea cheie nu părăsește niciodată instanțele tale. Releul află un hash care grupează conexiunile, pentru ce instanță este destinat un mesaj, cât de mare este și când trece. Un apel direct în rețeaua locală este sigilat la fel și semnat în plus, așa că nimic nu depinde de certificatul autosemnat pe care îl servește o instanță.

**Ce circulă prin grup.** Fișele de evaluare de pe pagina Instanțe, o cerere de a verifica un domeniu acum, ofertele Mesh pentru off-site, și ce are nevoie un receptor sau o sursă de preluare: locațiile de depozit ale celeilalte instanțe și parola ei restic. Datele de backup nu circulă niciodată așa, ele merg în continuare direct la backend-urile restic. Nici APP_KEY: parola restic deschide doar depozitele acelei instanțe și nimic altceva, nici secretele ei stocate, sesiunile sau codurile de recuperare.

**Intrări dinainte de împerechere.** Instanțele adăugate cu un token fleet, precum și receptorii și sursele de preluare configurate cu APP_KEY-ul celeilalte instanțe, rămân după actualizare și sunt marcați **Împerechează din nou**. Receptorii și sursele de preluare continuă să funcționeze: la prima pornire, BombVault înlocuiește fiecare APP_KEY stocat cu parola restic derivată din el. Împerechează ambele instanțe, apoi editează intrarea și alege instanța ei. O astfel de instanță își preia cardul vechi imediat ce o instanță cu același nume apare în grup.

Singurul loc care încă cere un APP_KEY manual este [Restaurare dintr-un alt depozit BombVault](#restore-from-another-bombvault-repo), pentru cazul în care cealaltă instanță a dispărut și nu mai poate răspunde în niciun grup.

## Panou de recepție (partea de recepție)

![Partea care primește, urmărită doar în citire, cu o verificare de integritate rulată pe această mașină.](assets/screenshots/receiver.png)

*Partea care primește, urmărită doar în citire, cu o verificare de integritate rulată pe această mașină.*

Tot ce este mai sus este partea de *trimitere*. Pe stația care **primește** copii off-site imuabile de la un alt BombVault, panoul de recepție îți oferă monitorizare independentă, doar în citire, a acelor depozite pe hardware-ul care primește, astfel încât o eșuare tăcută la capătul îndepărtat să nu treacă neobservată.

Activează comutatorul **Receptor** în Setări pentru a dezvălui o filă **Receptor**. Este oprit implicit; activează-l doar pe o stație care primește efectiv backupuri off-site imuabile. Apoi înregistrează un depozit primit (doar în citire, deschis cu parola restic a instanței care trimite, primită prin [grupul de împerechere](#pairing)) pentru a obține:

- **Un inventar de instantanee grupat pe sursă**, astfel încât să poți vedea exact care containere, VM-uri și seturi de fișiere au sosit.
- **Ultima primire** per sursă, astfel încât să știi cât de proaspătă este fiecare.
- **Un `restic check` independent** rulat pe hardware-ul care primește, astfel încât integritatea este verificată acolo unde stau efectiv datele, nu doar pe expeditor.
- **Un comutator de tip dead-man:** o alertă când o sursă încetează să trimită într-o fereastră pe care o setezi.
- **Alerte de integritate:** o alertă când o verificare pe partea de recepție eșuează.

Receptorul este strict doar în citire. Nu scrie niciodată în depozitul primit, deci nu poate niciodată strica garanția append-only pe care se bazează expeditorul.

## Exemplu complet: două mașini Unraid, de la un capăt la altul

Mai sus sunt descrise piesele. Aici este o configurație completă cu valori reale, pentru că piesele se asamblează mai ușor după ce le-ai văzut asamblate o dată.

Două mașini: **TOWER** rulează containerele și trimite copiile, **VAULT** le primește și impune imutabilitatea. Înlocuiește cu propriile nume, adrese și căi de partajare.

**1. Pe VAULT, ridică serverul append-only.** În BombVault pe TOWER mergi la *Setări → Extern → Configurează*, alege **rest-server** și generează rețeta. Copiază fila **Șablon Unraid (XML)**, salveaz-o pe VAULT ca `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, apoi *Docker → Add Container* și alege **rest-server** din lista de șabloane. Înainte de pornire, scrie linia `htpasswd` afișată în `/mnt/user/appdata/rest-server/.htpasswd` pe VAULT. Parola de unică folosință este afișată o singură dată și nu este niciodată păstrată: copiaz-o acum. Acea linie poartă aceeași parolă, deja criptată cu bcrypt pentru tine: textul simplu merge în datele de acces REST pe TOWER, linia criptată în `.htpasswd` pe VAULT. Nu trebuie să criptezi nimic tu.

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

**4. Pe VAULT, urmărește ce sosește.** Împerechează cele două stații ([Împerecherea instanțelor](#pairing)), activează *Setări → General → Receptor*, deschide fila **Receptor** și înregistrează depozitul doar pentru citire, cu TOWER ca instanță expeditoare.

!!! warning "Locația este o cale **din interiorul** containerului, scrisă relativ la montarea gazdei"
    Introdu `user/appdata/rest-server/bombvault-containers/containers`, **nu** `/mnt/user/appdata/…`. BombVault rulează într-un container unde `/mnt` al gazdei este montat în altă parte; o cale absolută a gazdei nu există acolo. Dacă lipești una, BombVault îți spune acum ce cale relativă să folosești.

    VAULT primește parola restic de la TOWER prin grup la salvare; nimeni nu trebuie să tasteze o cheie.

**5. Fă-l reciproc, dacă vrei.** Repetă aceiași cinci pași în sens invers: un rest-server pe TOWER care primește copia VAULT. Atunci fiecare mașină impune imutabilitatea pentru cealaltă, și niciuna nu poate șterge copiile celeilalte.

## Recuperare ghidată

O filă dedicată **Recuperare** conduce o instalare nouă sau reconstruită prin cazul de dezastru, într-un singur loc:

1. **Restaurează mai întâi propriile setări ale BombVault**, astfel încât căile de backup, țintele off-site și credențialele de care restul fluxului are nevoie să fie precompletate (aplicate printr-o auto-repornire peste socket-ul Docker, astfel încât baza de date de setări în execuție să nu fie niciodată suprascrisă sub un handle deschis).
2. **Verifică dacă BombVault poate citi backupurile tale** (capcana cheii de criptare, în față).
3. Îți permite să **îndrepți către depozitul tău existent** (local sau off-site).
4. **Descoperă** containerele, VM-urile, seturile de fișiere și seturile de date ZFS stocate în el.
5. **Restaurează containerele și VM-urile dintr-odată** (lăsate oprite, ca să le pornești deliberat) și listează seturile de fișiere și elementele ZFS de restaurat unul câte unul; elementele ZFS revin dezactivate. Kitul tău de recuperare e la un clic distanță.

!!! tip "Migrare planificată versus dezastru"
    Recuperarea ghidată restaurează propriile setări ale BombVault dintr-un backup. Pentru o mutare *planificată* pe o stație nouă, poți în schimb să-ți muți configurația direct cu cardul **Exportă / importă setările** (un fișier JSON portabil). Vezi [Configurare](configuration.md#portable-settings-export-and-import).

### Restaurare dintr-un alt depozit BombVault {#restore-from-another-bombvault-repo}

Un card separat în fila **Recuperare** deschide depozitul unei *alte* instanțe BombVault (o partajare montată sub `/mnt`, sau un URL la distanță) cu **`APP_KEY`-ul acelei instanțe**, într-o sesiune unică, doar în citire. Răsfoiește containerele, VM-urile și seturile de fișiere stocate acolo, alege un instantaneu și restaurează-l, iar obiectul restaurat devine un container, VM sau set de fișiere local normal. Nimic nu este scris vreodată în celălalt depozit, iar propriile tale setări de backup rămân neatinse (sesiunea trăiește în memorie și expiră singură). Mutarea unui container de pe serverul A pe serverul B nu mai înseamnă repointarea setărilor depozitului tău și revenirea lor ulterioară. Acest card este de unică folosință: deschide o sesiune, restaurează ce alegi și uită cealaltă instanță. Dacă vrei în schimb un aranjament permanent, în care această stație aduce după o programare instantaneele altei instanțe în propriul depozit, aceea este fila **Preluare** a paginii **Instanțe**.

## Kit de recuperare a cheii de criptare

Aceasta este piesa care face recuperarea în caz de dezastru posibilă chiar și când nu există niciun BombVault în execuție.

Un clic descarcă **cheia principală**, **parola restic derivată** și **locațiile și comenzile exacte ale depozitului**, astfel încât să poți restaura direct cu CLI-ul restic pe orice mașină. O amintire pe panoul principal insistă până când l-ai stocat.

!!! danger "Stochează kitul de recuperare în afara serverului"
    Kitul conține secretul care decriptează backupurile tale. Păstrează-l undeva în siguranță și separat de server (un manager de parole, o copie printată într-un seif). Dacă pierzi atât BombVault cât și `APP_KEY` fără niciun kit de recuperare, backupurile tale criptate nu pot fi recuperate.

!!! warning "Cel mai nou snapshot nu este întotdeauna cel de restaurat"
    Începând cu restic 0.17, `restic snapshots` arată dimensiunea fiecărui snapshot. După o pierdere de date, cel mai nou snapshot poate fi cel golit, așa că nu restaura un snapshot mult mai mic decât cele dinaintea lui. După un ransomware poate fi cel criptat, de dimensiune obișnuită. Dacă BombVault încă rulează, uită-te mai întâi pe pagina sa **Anomalii**: ea numește ultimul backup bun. O restaurare nu are nevoie de niciun fel de date despre anomalii din BombVault, iar pauza retenției doar păstrează mai multe snapshoturi.

### Sigilarea kitului

Dacă ai activat criptarea age pentru exporturile în clar (Setări), kitul este sigilat și el și se descarcă drept `bombvault-recovery-kit.md.age`. Este în format ASCII armor, nu binar, așa că rămâne text: lipirea lui într-un manager de parole sau tipărirea funcționează exact ca înainte, doar că fără cheia ta conținutul nu poate fi citit.

!!! warning "Nu păstra cheia age în interiorul kitului"
    Ai nevoie de cheia age **privată** ca să deschizi un kit sigilat. Păstreaz-o într-un loc care nu depinde de kitul însuși, altfel vei avea două lucruri de recuperat în loc de unul. Sigilarea merită când kitul este păstrat într-un loc pe care nu îl controlezi pe deplin (un manager de parole partajat, notițe în cloud, o copie tipărită la birou); un kit din propriul tău seif este deja protejat de seif.

    Cu criptarea activată și niciun destinatar utilizabil configurat, descărcarea este refuzată de-a dreptul. BombVault nu recurge niciodată la predarea cheii principale în clar.

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
