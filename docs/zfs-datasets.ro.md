# Seturi de date ZFS

Seturile de date ZFS sunt elemente în lista **Copii de rezervă**, ca containerele și VM-urile. Un element este un set de date împreună cu fiecare set de date de sub el. La fiecare copie BombVault face un singur instantaneu ZFS al întregului arbore, așa că fiecare set de date din el este captat în același moment. Apoi citește fișierele fiecărui set de date din acel instantaneu și le stochează cu restic, la fel ca pe un folder, și șterge instantaneul imediat după. Copiile sunt deduplicate, poți răsfoi fiecare copie, iar fișierele individuale pot fi restaurate.

Copia nu trimite niciodată un flux în restic și nu readuce niciodată un set de date la o stare anterioară. BombVault distruge doar instantanee create chiar de el. [Replica](#replica) opțională este singurul loc care folosește `zfs send`: copiază seturile de date pe un al doilea server ZFS și nu atinge copia de rezervă.

## Cerințe {#requirements}

- **Legătura SSH cu acest server.** Seturile de date ZFS folosesc aceeași cheie, aceeași gazdă și același utilizator ca backupurile VM. Dacă backupurile VM funcționează deja, funcționează și asta. Altfel, urmează [ghidul de backup VM prin SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) de pe GitHub. Câmpurile șablonului se numesc **Host SSH: Address**, **Host SSH: Port** și **Host SSH: User**.
- **Comanda `zfs` pe acea gazdă.** Unraid 6.12 și mai nou, precum și TrueNAS SCALE, o au.
- **Host Data mapat ca `/mnt` cu Access Mode Read/Write - Slave.** Aceasta e valoarea implicită a șablonului. Instantaneul unui set de date apare în dosarul `.zfs/snapshot` al setului abia după ce BombVault a pornit, așa că containerul trebuie să primească montările pe care gazda le face mai târziu.
- **Seturile de date montate sub `/mnt`.** Pe Unraid, pool-urile se află în `/mnt/<pool>`, deci așa este deja.

Pornește secțiunea la **Setări, General** (Seturi de date ZFS). **Conexiune ZFS la acest server** de la **Mai mult** pe pagina Copii de rezervă deschide apoi **Conexiunea cu acest server**. Testează legătura SSH, numește utilizatorul și gazda la care se conectează și spune ce lipsește când ceva lipsește. Când legătura este căzută, lista Copii de rezervă spune asta cu **Vezi conexiunea**, iar cardul **Integrare gazdă** din prezentare arată același rezultat.

## Elemente și seturi de date copil {#items-and-children}

Alege **Adaugă** pe pagina Copii de rezervă, apoi **Un set de date ZFS**. Lista vine de la server. Alege setul de date din vârful a ceea ce vrei salvat, de exemplu `cache/appdata`, iar elementul îl acoperă pe el și fiecare set de date de sub el.

- **Seturile de date copil noi se alătură singure.** Un set de date creat ulterior sub element este salvat la rularea următoare, iar acea rulare îl numește ca nou. Prima lui copie îl citește integral o dată; după aceea se citesc doar modificările.
- **Poți lăsa deoparte copii individuali.** Oprește un copil în fila Setări a elementului și el este lăsat deoparte împreună cu tot ce e sub el. Un copil lăsat deoparte care nu mai există pe server este marcat ca atare și poate fi scos din listă.
- **Copiii care nu pot fi citiți sunt sărit, niciodată în tăcere.** Rularea îi listează, elementul arată câți au fost sărit, iar cardul **Fără backup** din prezentare numără fiecare ca neprotejat. Rularea salvează tot restul și nu eșuează din cauza unui copil sărit. Motivele sunt în [tabelul codurilor de motiv](#reason-codes): un set de date care nu este montat, are `canmount=off`, un punct de montare `legacy` sau lipsă, o cheie de criptare neîncărcată, accesul la instantanee oprit sau un punct de montare pe care BombVault nu îl vede.
- **Un set de date sărit nu își trage copiii după el.** Un set de date cu `canmount=off` care conține doar alte seturi de date este sărit (afișat ca "doar structură"), iar copiii lui montați sunt salvați. Un set de date criptat a cărui cheie nu e încărcată este sărit împreună cu copiii care îi împart cheia.
- **Copiii care sunt discuri de VM sau date de sistem pornesc dezactivați** în dialogul de adăugare, cu motivul lângă comutator. Adăugarea unui pool întreg cere o confirmare care enumeră ce conține.

### Volume {#volumes}

Un volum (zvol) conține un disc virtual, nu fișiere, iar un element ZFS nu face niciodată copie unuia.

- Un volum folosit de un VM este salvat împreună cu acel VM.
- Un volum pe care nu-l folosește niciun VM (un extent iSCSI, un disc pe care l-ai detașat) **nu este salvat de BombVault**. Fereastra de adăugare și pagina Copii de rezervă numără aceste volume și spun asta. O versiune viitoare le va salva.

Volumele din arborele unui element sunt sărite și numite la fiecare rulare.

### Stocarea Docker {#docker-storage}

Cu driverul de stocare ZFS al Docker, fiecare strat de imagine este un set de date cu punct de montare `legacy`. Dialogul de adăugare le adună într-o singură linie per părinte. Un arbore care conține mai mult de 20 de astfel de seturi de date nu poate deveni element: cât timp există un instantaneu al lui, Docker nu poate elimina straturi de imagine. Adaugă în schimb seturile de date de sub el, de exemplu `appdata`.

### Elementele nu se suprapun niciodată {#overlap}

Un set de date poate aparține unui singur element. BombVault refuză un element nou care se află în interiorul unuia existent sau care ar conține unul. Ca să unești mai multe elemente copil într-un element părinte, șterge întâi elementele copil și alege să le păstrezi copiile, apoi adaugă părintele. Fiecare set de date își păstrează istoricul sub propriul nume, așa că următoarea copie continuă de unde au rămas elementele vechi și nu citește totul din nou.

## Oprirea containerelor și comenzi în jurul instantaneului {#consistency}

Un instantaneu al unei baze de date în execuție seamănă cu o pană bruscă de curent: baza de date de obicei își revine, dar trebuie să o facă. Fiecare element poate face două lucruri în privința asta, și ambele acoperă doar momentul instantaneului, nu întreaga copie.

- **Oprește aceste containere pentru instantaneu.** BombVault oprește containerele din listă, face instantaneul și le pornește din nou imediat. Containerele de pe același nivel de dependențe se opresc în paralel, întâi cele dependente, așa că întreaga fereastră durează de obicei câteva secunde; rularea arată cât a durat. Copia citește apoi instantaneul înghețat în timp ce aplicațiile rulează deja din nou. Sunt oprite doar containerele care rulau.
- **O comandă înainte și după instantaneu.** Rulează într-un container ales de tine, de exemplu pentru a descărca o bază de date în setul de date chiar înainte de instantaneu, fără să oprească nimic. Dacă comanda de dinainte de instantaneu eșuează, copia eșuează și nu se face niciun instantaneu. O comandă de după instantaneu care eșuează apare la rulare, dar nu face copia să eșueze.

Ce se întâmplă când ceva merge prost:

- Dacă un container nu poate fi oprit, BombVault le pornește pe cele pe care le-a oprit deja, iar copia eșuează numind containerul. Nu recurge niciodată la un instantaneu al aplicațiilor care rulează.
- Oprirea așteaptă terminarea unei copii de container în curs (până la 30 de minute la o rulare manuală, până la limita de timp a copiei la una programată), ca cele două să nu oprească și să pornească niciodată același container în același timp.
- Înainte să se oprească primul container, BombVault notează care dintre ele le oprește. Dacă BombVault este oprit forțat în interiorul ferestrei, el pornește acele containere din nou la următoarea pornire, trimite o notificare, iar elementul arată o notă roșie pentru orice container pe care nu l-a putut porni.

Copiile automate SQL ale bazelor de date (vezi [Funcționalități](features.md)) rulează cu copia proprie a unui container, nu cu un element ZFS. O bază de date al cărei container este salvat doar prin setul de date nu primește dump, așa că dă-i aici o comandă.

Un container poate fi pe această listă și, în același timp, salvat ca container. Datele lui sunt atunci stocate de două ori, în două depozite, iar copia totală îl oprește de două ori. Elementul spune asta.

## Restaurare {#restore}

Deschide elementul și alege **Restaurare**, sau alege-l pe pagina **Restaurare**. Fereastra de restaurare cere copia, apoi **Setul de date**. Implicit este setul de date de sus al elementului. La **Ce** restaurezi tot sau doar fișierele și folderele alese de tine. **Unde** oferă:

- **Restaurează în setul de date.** Fișierele din copie sunt scrise în punctul de montare al setului de date. Fișierele cu același nume sunt suprascrise, celelalte rămân. Setul de date nu este niciodată readus la o stare anterioară sau înlocuit. BombVault verifică dacă setul de date este montat, vizibil și inscriptibil, o dată înainte de început și din nou chiar înainte de scriere. Acolo unde în interior e montat un set de date copil, nu se scrie nimic: copilul își păstrează fișierele, proprietarul și permisiunile și este restaurat din propria copie.
- **Într-un folder.** Alege un folder sub `/mnt`. BombVault verifică dacă folderul este pe un pool sau o partajare montată și dacă există destul spațiu liber. Funcționează fără legătura SSH și pentru seturi de date care nu mai există.
- **Într-un set de date nou.** Dă numele unui set de date care nu există încă; fereastra sugerează `<set-de-date>-restored`. BombVault îl creează cu proprietățile ZFS stocate în copie și restaurează în el, vezi [Restaurare ca set de date nou](#new-dataset).

Alegerea **Toate seturile de date ale acestei copii** ca set de date scrie fiecare set de date al arborelui în propriul subfolder al folderului ales. Seturile de date sărite în acea copie sunt numite. Ca să restaurezi din depozitul altui BombVault, folosește **Dintr-un folder vechi de copii** pe pagina Restaurare; ea restaurează mereu într-un folder: toate seturile de date ale unei copii, fiecare în propriul subfolder, sau un set de date al arborelui, întreg sau fișiere alese.

Lista elementului cu containere de oprit este oferită și pentru o restaurare în setul de date. Acele containere rămân oprite pe toată durata restaurării, iar copiile de containere așteaptă între timp.

### Instantaneul de siguranță {#safety-snapshot}

Înainte să scrie într-un set de date, BombVault face un instantaneu ZFS al acelui set de date, numit `bombvault-prerestore-<ora>`. **Păstrează întâi un instantaneu ZFS al stării de acum** este pornit implicit; oprirea lui cere o a doua confirmare. Dacă instantaneul nu poate fi făcut, nu se restaurează nimic.

BombVault nu șterge niciodată singur un instantaneu de siguranță. Elementul le listează cu vârsta și dimensiunea lor, fiecare cu acțiunea **Șterge**, și avertizează când cel mai vechi are mai mult de 30 de zile, pentru că păstrează pe pool date șterse și modificate.

Ca să revii după o restaurare, copiază fișiere individuale din `.zfs/snapshot/bombvault-prerestore-<oră>` din interiorul setului de date. `zfs rollback <dataset>@bombvault-prerestore-<oră>` funcționează doar cât timp este cel mai nou instantaneu al acelui set de date. `zfs rollback -r` șterge fiecare instantaneu mai nou, inclusiv pe cele automate.

### Restaurare ca set de date nou {#new-dataset}

BombVault salvează la fiecare backup proprietățile ZFS setate local ale fiecărui set de date: compression, recordsize, quota, reservation, atime, xattr, acltype, casesensitivity și propriile tale proprietăți de utilizator. Valorile moștenite și cele doar pentru citire sunt lăsate deoparte, pentru că revin singure. Backupurile de dinainte ca BombVault să le salveze nu au niciuna.

- **Într-un set de date nou** rulează `zfs create` cu fiecare proprietate salvată. casesensitivity, normalization și utf8only pot fi setate doar așa. Cotele și rezervările se setează după fișiere, ca să nu le poată refuza. Punctul de montare este lăsat deoparte, ca să nu se ciocnească copia de original, la fel `canmount`, `readonly` și criptarea, ca restaurarea să poată scrie. Un set de date nou sub unul criptat îi preia criptarea. Setul de date de deasupra trebuie să existe. Dacă ceva eșuează după creare, setul de date nou rămâne pe server, pentru că BombVault nu distruge niciodată un set de date.
- **Restaurează în setul de date** arată proprietățile salvate lângă restaurare. **Setează și aceste proprietăți** le setează pe cele pe care un set de date existent le mai acceptă, înainte să fie scris vreun fișier. Cotele și rezervările se setează după fișiere, ca să nu le poată refuza. Fără acest comutator, setul de date își păstrează setările.

## Ce conține copia {#contents}

În copie: fișierele și dosarele fiecărui set de date salvat, cu proprietarul, permisiunile, marcajele de timp și atributele extinse așa cum le stochează restic, precum și proprietățile ZFS setate local ale fiecărui set de date.

Nu sunt în copie:

- proprietarul și permisiunile dosarului de sus al fiecărui set de date în sine (tot ce e sub el este inclus). O restaurare în setul de date lasă dosarul de sus existent așa cum e, o restaurare într-un dosar îl creează cu permisiunile `0755`;
- instantaneele ZFS existente;
- copiii care au fost săriți sau excluși;
- volumele.

Pentru a restaura pe un pool nou, creează poolul și restaurează fiecare set de date într-un set de date nou. Nu s-a verificat încă dacă ACL-urile NFSv4, așa cum le folosește TrueNAS pe seturile de date SMB, revin așa cum te aștepți, așa că testează o restaurare cu propriile date înainte să te bazezi pe ele.

## Seturi de date criptate {#encryption}

Un set de date criptat este salvat doar cât timp cheia lui este încărcată. Altfel este sărit cu un avertisment; încarcă cheia cu `zfs load-key` și montează setul de date. BombVault citește datele decriptate și le stochează în depozitul restic, care este criptat. Dacă ai dezactivat criptarea în BombVault, acel depozit nu este criptat.

## Replică {#replica}

O replică este o copie a seturilor de date ale unui element pe un al doilea server ZFS. BombVault o ține la zi cu `zfs send` și `zfs receive`. Prima rulare trimite tot, după aceea circulă doar blocurile modificate. Pe celălalt server poți monta imediat copia.

O replică nu înlocuiește niciodată copia de rezervă. Versiunile mai vechi, fișierele individuale și verificarea vin tot din copii, iar replica păstrează doar atâtea instantanee câte stabilești. O replică actuală contează ca o copie în afara sediului, dar un element cu replică și fără copie de rezervă rămâne portocaliu.

Pornește-o în cardul **Replică** din fila Setări a elementului. Acolo alegi unde merge replica, când rulează (**După fiecare copie de rezervă** sau **Plan propriu**) și câte instantanee rămân pe țintă. Cardul listează fiecare set de date și volum cu starea lui, iar **Replică acum** pornește o rulare. O rulare de replică are un lock propriu, așa că un prim transfer lung nu oprește niciodată copiile de rezervă.

### Trimitere către un server ZFS {#replica-push}

Orice mașină cu ZFS și SSH poate primi, de exemplu un al doilea Unraid sau un TrueNAS. BombVault nu trebuie să ruleze acolo.

1. La **Setări, Împerechere**, alege **Conectează prin SSH** în **Server ZFS fără BombVault**, sau **Adaugă server ZFS** în cardul **Replică** al elementului.
2. Introdu adresa, utilizatorul și portul. Dialogul arată cheia publică a lui BombVault. Adaugă-o în `~/.ssh/authorized_keys` al utilizatorului pe server. Pe Unraid se află la **Users, root, SSH keys**. În funcție de **Display Settings**, Users este în meniul din antet sau la Settings. Aceasta este cheia replicii. Este diferită de cheia conexiunii SSH pentru VM-uri și gazdă, așa că adaug-o chiar dacă cealaltă este deja autorizată.
3. Testează conexiunea. Dialogul listează apoi pool-urile serverului. Alege unul și setează rădăcina, care implicit este `<pool>/bombvault-replica`.
4. Alege noul server în cardul **Replică** al elementului. Serverul are apoi un card propriu pe pagina Instanțe, iar **Deschide** duce la pagina lui.

Cu root nu mai trebuie nimic. Un utilizator propriu are nevoie de aceste permisiuni pe pool-ul țintei, pe care le arată și dialogul:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

Pe sursă, același fel de utilizator are nevoie de acestea pe setul de date de sus al elementului:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

În această direcție BombVault-ul care deține elementul deține și o cheie care poate scrie pe server.

BombVault reține cheia de gazdă pe care serverul o arată la prima conexiune și mai târziu refuză orice altă cheie. Dacă serverul a fost reinstalat, apasă **Uită cheia de gazdă** pe pagina serverului, iar conexiunea următoare va reține cheia nouă.

### Trimitere către o instanță împerecheată {#replica-receive}

Un BombVault împerecheat poate primi singur replica. Nimeni nu primește acces SSH la celălalt host și nicio cheie nu ajunge într-un fișier `authorized_keys`.

1. În cardul **Replică** al elementului, alege instanța împerecheată drept țintă sau pornește rolul **Server ZFS** pe cardul acelei instanțe de pe pagina Instanțe și alege acolo elementele ZFS. Cardul arată **Așteaptă aprobarea** până răspunde.
2. Pe instanța care primește, deschide **Instanțe**. Cererea apare pe cardul instanței care trimite ca insignă, iar **Detalii** o listează. Alege pool-ul și rădăcina de pe acel server și câte instantanee rămân, apoi apasă **Permite** sau **Refuză**.
3. După **Permite**, sursa trimite după propriul plan, ca la orice altă țintă.

Instanța care primește acceptă doar ce acoperă aprobarea: seturile de date ale elementului, în propria rădăcină. Rulează ea însăși `zfs receive`, iar sursa nu are cum să șteargă sau să deruleze înapoi ceva acolo. Astfel copia supraviețuiește unei surse pe care a preluat-o cineva. Instanța care primește își păstrează propria retenție. Sursa doar sugerează o regulă când cere.

**Revoke access** pe instanța care primește încheie oricând aprobarea și anunță sursa, care arată atunci că aprobarea a fost retrasă și se oprește. Ce deține deja instanța care primește rămâne acolo. O cerere refuzată rămâne refuzată. Mai multe seturi de date, sau o cerere după o revocare, întreabă din nou.

Instanța care primește trebuie să răspundă prin HTTPS. La o adresă http simplă sursa nu trimite nimic, pentru că tokenul și datele ar trece prin rețea necriptate. Fiecare transfer lasă în plus liberă o zecime din pool-ul care primește și se oprește înainte să se atingă de ea, așa că o singură sursă nu poate umple pool-ul serverului care primește.

### Unde ajung datele {#replica-target}

Fiecare set de date ajunge la `<root>/<server>/<pool>/<path>`. Dosarul serverului este numele instanței sursă, fixat la primul transfer, deci două servere cu același nume de pool nu se încurcă niciodată. De exemplu, `cache/appdata` al unui server numit `tower` ajunge la `backup/bombvault-replica/tower/cache/appdata`.

Copia de pe țintă este doar în citire și nu este montată, deci nu acoperă niciodată nimic pe acel server. Proprietățile ZFS merg cu ea, cu excepția punctului de montare, `sharenfs` și `sharesmb`. Nici rezervările `reservation` și `refreservation` nu sunt preluate, așa că copia ocupă doar spațiul de care au nevoie datele ei.

### Ce intră {#replica-contents}

Tot ce salvează elementul intră, iar și volumele de sub el, pe care copia de rezervă le sare. Un set de date copil pe care l-ai oprit în element rămâne în afară. Toate seturile de date ale unei rulări provin dintr-un singur instantaneu, ca la copia de rezervă.

### Cât rămân instantaneele {#replica-retention}

Pe țintă, o replică nouă păstrează 7 instantanee zilnice și 3 săptămânale. Alege în loc **Short**, **Balanced** sau **Long**, sau setează **Custom values**. Acolo se șterg doar instantaneele numite `bombvault-replica-<14 digits>`, și niciodată cel mai nou pe care îl au în comun ambele părți.

Pe sursă, BombVault păstrează doar ultimul instantaneu de replică, plus un semn de carte pentru fiecare stare trimisă. Semnele de carte nu ocupă spațiu. Următorul transfer pornește de la ele.

### Seturi de date criptate într-o replică {#replica-encryption}

Un set de date criptat se trimite brut. Rămâne criptat pe țintă, iar ținta nu vede niciodată cheia. Ține cheia în siguranță: ai nevoie de ea ca să deschizi copia după o restaurare, iar o replică fără ea nu se poate citi.

### Folosirea replicii {#replica-use}

Deschide fila **Copii de rezervă** a elementului și dă clic pe rândul **Replică ZFS** printre locurile care îi păstrează instantaneele. Foaia listează instantaneele de pe țintă și arată comenzile cu numele tale reale.

Ca să vezi o stare veche, clonează un instantaneu pe țintă. O clonă nu ocupă spațiu până nu se schimbă ceva, iar replica rămâne neatinsă:

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Dacă sursa cedează, transformă copia într-un set de date obișnuit, care poate fi scris, pe țintă:

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs inherit -r canmount backup/bombvault-replica/tower/cache/appdata && zfs mount -a
```

După aceea, BombVault nu mai replică pe acel set de date, până pornești o nouă primă rulare.

Ca să aduci o stare înapoi pe sursă, apasă **Bring back as a new dataset** în foaie. BombVault trimite instantaneul într-un set de date nou, lângă cel original, cu numele `<dataset>-bombvault-restore-` plus o marcă de timp. Nu scrie niciodată peste original.

## Instantanee rămase {#leftover-snapshots}

Instantaneul unei copii se numește `<dataset>@bombvault-<14 cifre>`, de exemplu `cache/appdata@bombvault-20260924021500` (UTC). BombVault îl elimină imediat după copie. Dacă asta nu reușește, de exemplu pentru că setul de date e ocupat sau BombVault a fost oprit, BombVault îl elimină:

- înainte de următoarea copie a acelui element,
- la pornirea BombVault, pentru fiecare element, chiar și cu secțiunea dezactivată,
- când ștergi elementul,
- când apeși **Scoate acum** la element, care arată și câte au mai rămas.

Sunt eliminate doar numele care corespund exact cu `bombvault-` plus 14 cifre. Instantaneele de siguranță, propriile tale instantanee și cele automate nu sunt atinse niciodată. Ca să elimini unul manual:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

Instantaneul unei replici se numește `<dataset>@bombvault-replica-<14 digits>` și nu este o rămășiță. Rămâne pe sursă până când următoarea rulare de replică îl înlocuiește, iar pe țintă cât timp îl păstrează retenția. Curățătorul nu îl atinge niciodată, pentru că se potrivește doar cu `bombvault-` urmat de exact 14 cifre.

## Anomalii {#anomalies}

Un copil golit schimbă abia totalul unui arbore mare, așa că detecția anomaliilor urmărește fiecare set de date al unui element separat: dimensiunea, numărul de fișiere, datele noi și timpul restic au fiecare istoricul lor. Acel istoric aparține numelui setului de date, așa că rămâne când arborele este salvat mai târziu de alt element.

Un set de date pe care rularea anterioară l-a salvat și această rulare nu l-a putut citi contează ca golit, atât timp cât selecția elementului nu s-a schimbat. Asta acoperă o cheie neîncărcată, un set de date nemontat și unul dispărut din arbore. Un copil pe care îl excluzi tu schimbă selecția, așa că istoricul lui începe de la zero. Cât timp o constatare despre date pierdute este deschisă, retenția păstrează copiile vechi ale acelui set de date și curăță restul arborelui ca de obicei.

Fiecare set de date are o linie proprie în monitorizarea elementului, care se deschide cu **Monitorizare** pe pagina **Anomalii** și stă în fila **Verificare** a elementului. Constatările deschise apar lângă fiecare set de date. Linkul dintr-o constatare deschide fereastra de restaurare pe ultima copie bună a setului de date. Dacă o rulare se încheie se judecă pentru întregul element, pentru că o rulare reușește sau eșuează ca întreg.

Verificările în sine sunt descrise la [Funcționalități](features.md). Un asistent conectat prin [serverul MCP](mcp.md) poate lista punctele de restaurare ale unui element ZFS, îi poate porni copia și poate citi constatările, dar confirmarea unei constatări se face pe pagina **Anomalii**.

## Coduri de motiv {#reason-codes}

Pagina elementului, istoricul rulărilor și notificările numesc o problemă cu unul dintre aceste coduri. Cele mai multe au și remedierea lângă ele pe pagină.

| Cod | Înțeles | Ce faci |
|---|---|---|
| `ssh-missing` | Conexiunea SSH nu este configurată în acest container. | Configurează legătura SSH ca pentru backupurile VM. |
| `host-placeholder` | Host SSH: Address este încă valoarea de exemplu, iar `host.docker.internal` nu a răspuns nici el. | Setează Host SSH: Address la IP-ul LAN al acestui server. |
| `host-fallback` | Host SSH: Address este încă valoarea de exemplu, iar `host.docker.internal` funcționează. | Nimic, sau setează IP-ul LAN. |
| `ssh-unreachable` | Serverul nu poate fi atins prin SSH. | Verifică adresa și portul și dacă SSH este pornit. |
| `ssh-auth` | Serverul a refuzat cheia BombVault. | Rulează o dată pe server comanda arătată la **Conexiunea cu acest server**. |
| `zfs-not-found` | Gazda SSH nu are comanda `zfs`. | Îndreaptă Host SSH: Address spre mașina care deține pool-urile. |
| `zfs-permission` | Utilizatorul SSH nu are voie să ruleze această comandă zfs. | Folosește root sau vezi [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` numește altă gazdă sau alt utilizator decât câmpurile SSH. | Pune-le de acord sau golește câmpurile SSH, ca ambele să vină din URI. |
| `zfs-error` | zfs a raportat altă eroare. | Detaliile arată mesajul lui. |
| `propagation-missing` | Montările noi de pe gazdă nu ajung la container. | Setează Access Mode pentru Host Data la Read/Write - Slave și repornește BombVault. |
| `invalid-name` | Un nume de set de date pe care BombVault nu îl acceptă. | Redenumește setul de date. |
| `name-too-long` | Un set de date din arbore este prea lung pentru un nume de instantaneu. | Redenumește-l sau adaugă ca element un set de date de sub el. |
| `invalid-exclude` | Un tipar de excludere sau un copil lăsat deoparte nu se potrivește elementului. | Corectează elementul numit în mesaj. Ca să lași deoparte un set de date copil întreg, oprește-l în loc să scrii un tipar. |
| `not-found` | Setul de date nu există pe server. | Elimină elementul sau recreează setul de date. Copiile lui rămân restaurabile. |
| `not-filesystem` | Acesta e un volum, nu un sistem de fișiere. | Vezi [Volume](#volumes). |
| `overlaps-item` | Setul de date se suprapune cu un element existent. | Vezi [Elementele nu se suprapun niciodată](#overlap). |
| `docker-storage` | Arborele conține stocarea de imagini Docker. | Vezi [Stocarea Docker](#docker-storage). |
| `nothing-readable` | Niciun set de date din element nu poate fi citit acum. | Uită-te la codurile seturilor de date sărite. |
| `snapshot-failed` | Instantaneul nu a putut fi creat. | Detaliile arată mesajul zfs. |
| `containers-busy` | O copie de container încă rula când containerele trebuiau oprite. | Pornește din nou mai târziu. Rulările programate așteaptă singure. |
| `consistency-stop-failed` | Un container nu a putut fi oprit, așa că nu s-a făcut niciun instantaneu. | Verifică containerul sau scoate-l din listă. |
| `pre-snapshot-failed` | Comanda de dinainte de instantaneu a eșuat. | Detaliile rulării arată ieșirea ei. |
| `container-unknown` | Un container din listă nu există. | Scoate-l din listă. |
| `container-is-self` | BombVault nu își poate opri propriul container. | Scoate-l din listă. |
| `leftover-snapshots` | Pe server mai sunt instantanee pe care BombVault nu le-a putut elimina. | Apasă **Scoate acum**, vezi [Instantanee rămase](#leftover-snapshots). |
| `zvol` | Un volum în arbore, sărit. | Vezi [Volume](#volumes). |
| `canmount-off` | Niciodată montat (`canmount=off`), sărit. | Dacă are date, montează-l sau mută datele într-un set de date copil. |
| `legacy-mount` | Punct de montare legacy, sărit. | Dă-i un punct de montare sub `/mnt`. |
| `no-mountpoint` | Fără punct de montare, sărit. | Dă-i un punct de montare sub `/mnt`. |
| `not-mounted` | Nemontat pe server, sărit. | Montează-l cu `zfs mount` sau setează `canmount=on`. |
| `key-not-loaded` | Criptat și cheia nu este încărcată, sărit. | `zfs load-key`, apoi montează-l. |
| `snapdir-disabled` | Accesul la instantanee este dezactivat, sărit. | `zfs set snapdir=hidden <dataset>`. Dosarul `.zfs` rămâne ascuns. |
| `not-visible` | BombVault nu vede punctul de montare al setului de date. | Mută punctul de montare sub calea Host Data sau mapează-l în container la aceeași cale cu Read/Write - Slave. |
| `shfs-only` | Setul de date e vizibil doar prin `/mnt/user`, care ascunde instantaneele. | Mapează ca Host Data `/mnt`, nu `/mnt/user`. |
| `snapshot-not-visible` | Instantaneul a fost creat, dar nu a apărut în BombVault. | Rulează **Testează accesul la instantanee**; vezi mai jos. |
| `snapshot-loop` | Instantaneul nu a ajuns la BombVault pentru că Host Data nu lasă să treacă montările noi. | Setează Access Mode pentru Host Data la Read/Write - Slave și repornește BombVault. |
| `backup-failed` | restic a eșuat pentru acest set de date. | Detaliile rulării arată de ce. |
| `not-reached` | Rularea s-a încheiat înainte de acest set de date. | Rulează din nou copia. |
| `gone` | Setul de date nu mai este pe server. | Nimic. Copiile lui rămân restaurabile. |
| `read-only-mount` | BombVault poate doar să citească setul de date, deci nu poate restaura în el. | Setează maparea la Read/Write - Slave sau restaurează într-un dosar. |
| `destination-not-mounted` | Dosarul nu este pe un pool sau o partajare montată. | Alege un dosar de pe un pool sau o partajare. |
| `not-enough-space` | Nu e destul spațiu liber la destinație. | Eliberează spațiu sau alege alt dosar. |
| `safety-snapshot-failed` | Instantaneul de siguranță nu a putut fi făcut, așa că nu s-a restaurat nimic. | Detaliile arată mesajul zfs. |
| `safety-name-too-long` | Numele setului de date este prea lung pentru un instantaneu de siguranță. | Dezactivează instantaneul de siguranță sau restaurează într-un dosar. |
| `dataset-exists` | Există deja un set de date cu acest nume. | Alege un nume nou sau restaurează chiar în setul de date. |
| `create-failed` | Setul de date nou nu a putut fi creat. | Detaliile arată mesajul zfs. Verifică dacă setul de date de deasupra există. |
| `new-dataset-not-visible` | Setul de date nou a fost creat, dar BombVault nu îl vede, așa că nu s-a restaurat nimic. | Setul de date rămâne pe server. Montează-l sub calea Host Data și restaurează în el. |
| `set-properties-failed` | Proprietățile salvate nu au putut fi setate, așa că nu s-a restaurat nimic. | Detaliile arată mesajul zfs. |
| `set-limits-failed` | Fișierele au fost restaurate, dar cota sau rezervarea salvată nu a putut fi setată. | Detaliile arată mesajul zfs. Setează singur cota sau rezervarea cu `zfs set`. |

### Verifică ce vede containerul {#mountinfo}

**Testează accesul la instantanee** în fila Setări a elementului face un instantaneu real al arborelui lui, îl caută în BombVault pentru fiecare set de date și îl șterge din nou. Este cea mai rapidă cale de a dovedi tot traseul înainte de prima rulare programată.

Ca să te uiți singur, rulează asta pe server:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Fiecare linie este o montare în container. Linia unui set de date arată calea lui în container (sub `/host/user`) și numele setului de date. Un câmp `master:N` pe acea linie înseamnă că montarea primește montările pe care gazda le face mai târziu, iar de asta are nevoie accesul la instantanee. Dacă lipsește, setează Access Mode pentru Host Data la Read/Write - Slave și repornește BombVault.

## TrueNAS SCALE {#truenas}

- Când `LIBVIRT_URI` este setat (ca pentru backupurile VM pe TrueNAS), BombVault ia din URI gazda, utilizatorul și portul SSH pentru comenzile lui zfs, pe fiecare care nu e setat separat. Fără backupuri VM, setează în schimb `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` și `LIBVIRT_SSH_PORT`. Adaugă variabilele în **Additional Environment Variables**.
- Un utilizator altul decât root are nevoie de permisiune pe setul de date de sus al elementului, care o acoperă apoi pe a fiecărui set de date de sub el:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Restaurarea într-un set de date nou are nevoie în plus de `create` pe setul de date de deasupra, iar setarea proprietăților salvate are nevoie de drepturi pentru acele proprietăți.

  O sesiune SSH fără root pe TrueNAS nu are `/usr/sbin` în cale; BombVault apelează atunci direct `/usr/sbin/zfs`.
- **Host Data** al aplicației trebuie să fie o cale a gazdei deasupra seturilor de date, de exemplu `/mnt/tank`, nu un ixVolume. Cu o cale a gazdei, aplicația transmite montările noi ale gazdei către BombVault (`rslave`), iar de asta are nevoie accesul la instantanee.
