# Seturi de date ZFS

Pagina **ZFS** face copii de rezervă ale seturilor de date ZFS. Un element este un set de date împreună cu toate seturile de date de sub el. Pentru fiecare copie, BombVault face un singur instantaneu ZFS al întregului arbore, așa că fiecare set de date din el este surprins în același moment. Apoi citește fișierele fiecărui set de date din acel instantaneu, le stochează cu restic la fel ca pe un dosar și elimină instantaneul imediat după. Copiile sunt deduplicate, le poți răsfoi pe fiecare, iar fișierele individuale pot fi restaurate.

BombVault nu folosește niciodată `zfs send` pentru seturi de date, nu readuce niciodată un set de date la o stare anterioară și nu distruge niciodată vreunul.

## Cerințe {#requirements}

- **Legătura SSH cu acest server.** Seturile de date ZFS folosesc aceeași cheie, aceeași gazdă și același utilizator ca backupurile VM. Dacă backupurile VM funcționează deja, funcționează și asta. Altfel, urmează [ghidul de backup VM prin SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) de pe GitHub. Câmpurile șablonului se numesc **Host SSH: Address**, **Host SSH: Port** și **Host SSH: User**.
- **Comanda `zfs` pe acea gazdă.** Unraid 6.12 și mai nou, precum și TrueNAS SCALE, o au.
- **Host Data mapat ca `/mnt` cu Access Mode Read/Write - Slave.** Aceasta e valoarea implicită a șablonului. Instantaneul unui set de date apare în dosarul `.zfs/snapshot` al setului abia după ce BombVault a pornit, așa că containerul trebuie să primească montările pe care gazda le face mai târziu.
- **Seturile de date montate sub `/mnt`.** Pe Unraid, pool-urile se află în `/mnt/<pool>`, deci așa este deja.

Activează domeniul în **Setări, General** (Seturi de date ZFS). Pagina ZFS arată atunci cardul **Conexiunea cu acest server**. Acesta testează legătura SSH, numește utilizatorul și gazda la care se conectează și spune ce lipsește atunci când lipsește ceva. Verificarea integrării cu gazda (`/spike`) arată același rezultat.

## Elemente și seturi de date copil {#items-and-children}

Deschide **Adaugă seturi de date** pe pagina ZFS. Lista vine de pe server. Alege setul de date aflat cel mai sus din ce vrei să salvezi, de exemplu `cache/appdata`, iar elementul îl acoperă pe el și fiecare set de date de sub el.

- **Seturile de date copil noi se adaugă singure.** Un set de date creat mai târziu sub element este salvat la următoarea rulare, care îl semnalează ca nou. Prima lui copie îl citește complet o dată; după aceea sunt citite doar modificările.
- **Poți lăsa pe dinafară copii individuali.** Dezactivează un copil în setările elementului și este lăsat pe dinafară împreună cu tot ce e sub el. Un copil exclus care nu mai există pe server este marcat ca atare și poate fi scos din listă.
- **Copiii care nu pot fi citiți sunt săriți, niciodată pe tăcute.** Rularea îi enumeră, elementul arată câți au fost săriți, iar cardul de acoperire din panou îl numără pe fiecare ca neprotejat. Rularea salvează totuși tot restul și nu eșuează din cauza unui copil sărit. Motivele sunt în [tabelul codurilor de motiv](#reason-codes): un set de date nemontat, cu `canmount=off`, cu punct de montare `legacy` sau fără, cu o cheie de criptare neîncărcată, cu accesul la instantanee dezactivat sau cu un punct de montare pe care BombVault nu îl vede.
- **Un set de date sărit nu își trage copiii după el.** Un set de date cu `canmount=off` care conține doar alte seturi de date este sărit (afișat ca "doar structură"), iar copiii lui montați sunt salvați. Un set de date criptat a cărui cheie nu e încărcată este sărit împreună cu copiii care îi împart cheia.
- **Copiii care sunt discuri de VM sau date de sistem pornesc dezactivați** în dialogul de adăugare, cu motivul lângă comutator. Adăugarea unui pool întreg cere o confirmare care enumeră ce conține.

### Volume {#volumes}

Un volum (zvol) conține un disc virtual în loc de fișiere, iar pagina ZFS nu salvează niciodată unul.

- Un volum folosit de o VM este salvat cu acea VM pe pagina **VM-uri**.
- Un volum pe care nu îl folosește nicio VM (un extent iSCSI, un disc deconectat) **nu este salvat de BombVault**. Dialogul de adăugare și pagina ZFS numără aceste volume și spun asta. O versiune ulterioară le va salva.

Volumele din arborele unui element sunt sărite și numite la fiecare rulare.

### Stocarea Docker {#docker-storage}

Cu driverul de stocare ZFS al Docker, fiecare strat de imagine este un set de date cu punct de montare `legacy`. Dialogul de adăugare le strânge într-o singură linie pe părinte. Un arbore cu mai mult de 20 de astfel de seturi de date nu poate deveni element: cât timp există un instantaneu al lui, Docker nu poate elimina straturi de imagine. Adaugă în schimb seturile de date de sub el, de exemplu `appdata`.

### Elementele nu se suprapun niciodată {#overlap}

Un set de date poate aparține unui singur element. BombVault refuză un element nou care se află în interiorul unuia existent sau care ar conține unul. Ca să unești mai multe elemente copil într-un element părinte, șterge mai întâi elementele copil alegând să le păstrezi copiile, apoi adaugă părintele. Fiecare set de date își păstrează istoricul sub propriul nume, așa că următoarea copie continuă de unde s-au oprit elementele vechi și nu recitește totul.

## Oprirea containerelor și comenzi în jurul instantaneului {#consistency}

Un instantaneu al unei baze de date care rulează e ca o pană bruscă de curent: baza de date își revine de obicei, dar trebuie să o facă. Fiecare element poate face două lucruri în privința asta, iar ambele acoperă doar momentul instantaneului, nu întreaga copie.

- **Oprește aceste containere pentru instantaneu.** BombVault oprește containerele din listă, face instantaneul și le pornește din nou imediat. Containerele de pe același nivel de dependențe se opresc în paralel, întâi cele dependente, așa că întreaga fereastră durează de obicei câteva secunde; rularea arată cât a durat. Copia citește apoi instantaneul înghețat în timp ce aplicațiile rulează deja din nou. Sunt oprite doar containerele care rulau.
- **O comandă înainte și după instantaneu.** Rulează într-un container ales de tine, de exemplu pentru a descărca o bază de date în setul de date chiar înainte de instantaneu, fără să oprească nimic. Dacă comanda de dinainte de instantaneu eșuează, copia eșuează și nu se face niciun instantaneu. O comandă de după instantaneu care eșuează apare la rulare, dar nu face copia să eșueze.

Ce se întâmplă când ceva merge prost:

- Dacă un container nu poate fi oprit, BombVault le pornește pe cele pe care le-a oprit deja, iar copia eșuează numind containerul. Nu recurge niciodată la un instantaneu al aplicațiilor care rulează.
- Oprirea așteaptă terminarea unei copii de container în curs (până la 30 de minute la o rulare manuală, până la limita de timp a copiei la una programată), ca cele două să nu oprească și să pornească niciodată același container în același timp.
- Înainte ca primul container să se oprească, BombVault notează pe care le oprește. Dacă BombVault este oprit forțat în timpul ferestrei, la următoarea pornire repornește acele containere, trimite o notificare, iar elementul arată o notă roșie pentru fiecare container pe care nu l-a putut porni.

Descărcările automate ale bazelor de date (vezi [Funcționalitățile](features.md)) rulează cu copia proprie a unui container pe pagina **Containere**, nu cu un element ZFS. O bază de date al cărei container e salvat doar prin setul lui de date nu primește descărcare, așa că dă-i aici o comandă.

Un container poate fi în această listă și pe pagina **Containere** în același timp. Datele lui sunt atunci stocate de două ori, în două depozite, iar **Backup total** îl oprește de două ori. Elementul semnalează asta.

## Restaurare {#restore}

Deschide **Copii de rezervă** la element, alege copia, apoi setul de date. Implicit este setul de date de sus al elementului.

- **Restaurează în setul de date.** Fișierele din copie sunt scrise în punctul de montare al setului de date. Fișierele cu același nume sunt suprascrise, celelalte rămân. Setul de date nu este niciodată readus la o stare anterioară sau înlocuit. BombVault verifică dacă setul de date este montat, vizibil și inscriptibil, o dată înainte de început și din nou chiar înainte de scriere. Acolo unde în interior e montat un set de date copil, nu se scrie nimic: copilul își păstrează fișierele, proprietarul și permisiunile și este restaurat din propria copie.
- **Restaurează într-un dosar.** Alege un dosar sub `/mnt`. BombVault verifică dacă dosarul e pe un pool sau o partajare montată și dacă există destul spațiu liber. Funcționează fără legătura SSH și pentru seturi de date care nu mai există.
- **Alege fișiere** (avansat): scrie înapoi în setul de date doar fișierele și dosarele pe care le alegi.
- **Toate seturile de date ale acestei copii** (avansat): fiecare set de date al arborelui în propriul subdosar al dosarului ales. Seturile de date sărite în acea copie sunt numite.
- **De pe alt server:** pagina **Recuperare** restaurează din depozitul unui alt BombVault, mereu într-un dosar: toate seturile de date ale unei copii, fiecare în propriul subdosar, sau un set de date al arborelui, întreg sau fișiere alese.

Lista de containere de oprit a elementului este oferită și pentru o restaurare în setul de date. Acele containere rămân oprite pe toată durata restaurării, iar copiile containerelor așteaptă între timp.

### Instantaneul de siguranță {#safety-snapshot}

Înainte să scrie într-un set de date, BombVault face un instantaneu ZFS doar al acelui set de date, numit `bombvault-prerestore-<oră>`. Este activat implicit; dezactivarea lui cere o a doua confirmare. Dacă instantaneul nu poate fi făcut, nu se restaurează nimic.

BombVault nu șterge niciodată singur un instantaneu de siguranță. Elementul le enumeră cu vechimea și dimensiunea, fiecare cu acțiunea **Șterge**, și avertizează când cel mai vechi are peste 30 de zile, pentru că reține în pool date șterse și modificate.

Ca să revii după o restaurare, copiază fișiere individuale din `.zfs/snapshot/bombvault-prerestore-<oră>` din interiorul setului de date. `zfs rollback <dataset>@bombvault-prerestore-<oră>` funcționează doar cât timp este cel mai nou instantaneu al acelui set de date. `zfs rollback -r` șterge fiecare instantaneu mai nou, inclusiv pe cele automate.

### Restaurare ca set de date nou {#new-dataset}

BombVault nu creează seturi de date. Creează-l pe server cu proprietățile dorite, apoi restaurează într-un dosar care este punctul lui de montare:

```
zfs create -o compression=lz4 cache/appdata-restored
```

iar în BombVault restaurează în dosarul `cache/appdata-restored` sub `/mnt`.

## Ce conține copia {#contents}

În copie: fișierele și dosarele fiecărui set de date salvat, cu proprietarul, permisiunile, marcajele de timp și atributele extinse așa cum le stochează restic.

Nu sunt în copie:

- proprietățile ZFS ale seturilor de date (compression, recordsize, quota, mountpoint și restul);
- proprietarul și permisiunile dosarului de sus al fiecărui set de date în sine (tot ce e sub el este inclus). O restaurare în setul de date lasă dosarul de sus existent așa cum e, o restaurare într-un dosar îl creează cu permisiunile `0755`;
- instantaneele ZFS existente;
- copiii care au fost săriți sau excluși;
- volumele.

Ca să restaurezi pe un pool nou, creează mai întâi seturile de date cu proprietățile dorite. Nu s-a verificat încă dacă ACL-urile NFSv4, așa cum le folosește TrueNAS pe seturile de date SMB, revin așa cum te aștepți, așa că testează o restaurare cu propriile date înainte să te bazezi pe ele.

## Seturi de date criptate {#encryption}

Un set de date criptat este salvat doar cât timp cheia lui este încărcată. Altfel este sărit cu un avertisment; încarcă cheia cu `zfs load-key` și montează setul de date. BombVault citește datele decriptate și le stochează în depozitul restic, care este criptat. Dacă ai dezactivat criptarea în BombVault, acel depozit nu este criptat.

## Instantanee rămase {#leftover-snapshots}

Instantaneul unei copii se numește `<dataset>@bombvault-<14 cifre>`, de exemplu `cache/appdata@bombvault-20260924021500` (UTC). BombVault îl elimină imediat după copie. Dacă asta nu reușește, de exemplu pentru că setul de date e ocupat sau BombVault a fost oprit, BombVault îl elimină:

- înainte de următoarea copie a acelui element,
- la pornirea BombVault, pentru fiecare element, chiar și cu domeniul dezactivat,
- când ștergi elementul,
- când apeși **Scoate acum** la element, care arată și câte au mai rămas.

Sunt eliminate doar numele care corespund exact cu `bombvault-` plus 14 cifre. Instantaneele de siguranță, propriile tale instantanee și cele automate nu sunt atinse niciodată. Ca să elimini unul manual:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

## Anomalii {#anomalies}

Un copil care a fost golit abia schimbă totalul unui arbore mare, așa că detectarea anomaliilor urmărește fiecare set de date al unui element separat: dimensiunea, numărul de fișiere, datele noi și timpul restic au fiecare propriul istoric. Acel istoric aparține numelui setului de date, așa că rămâne când arborele este salvat mai târziu de alt element.

Un set de date pe care rularea anterioară l-a salvat și pe care rularea aceasta nu l-a putut citi contează ca golit, cât timp selecția elementului nu s-a schimbat. Asta acoperă o cheie neîncărcată, un set de date nemontat și unul care a dispărut din arbore. Un copil pe care îl excluzi tu însuți schimbă selecția, așa că istoricul lui o ia de la capăt. Cât timp o constatare despre date pierdute e deschisă, retenția păstrează copiile vechi doar ale acelui set de date și curăță restul arborelui ca de obicei.

În fila **Elemente** a paginii **Anomalii**, fiecare set de date are propriul rând sub elementul său, iar arborele elementului de pe această pagină arată constatările deschise lângă fiecare set de date. Linkul dintr-o constatare deschide panoul de restaurare al elementului la ultima copie bună a setului de date. Dacă o rulare se termină se judecă pentru întregul element, pentru că o rulare reușește sau eșuează ca întreg.

Verificările propriu-zise sunt descrise în [Funcționalitățile](features.md). Un asistent conectat prin [serverul MCP](mcp.md) poate enumera punctele de restaurare ale unui element ZFS, îi poate porni copia și poate citi constatările, dar confirmarea unei constatări se face pe pagina **Anomalii**.

## Coduri de motiv {#reason-codes}

Pagina, istoricul rulărilor și notificările numesc o problemă cu unul dintre aceste coduri. Pentru majoritatea, soluția apare și alături, pe pagină.

| Cod | Înțeles | Ce faci |
|---|---|---|
| `ssh-missing` | Conexiunea SSH nu este configurată în acest container. | Configurează legătura SSH ca pentru backupurile VM. |
| `host-placeholder` | Host SSH: Address este încă valoarea de exemplu, iar `host.docker.internal` nu a răspuns nici el. | Setează Host SSH: Address la IP-ul LAN al acestui server. |
| `host-fallback` | Host SSH: Address este încă valoarea de exemplu, iar `host.docker.internal` funcționează. | Nimic, sau setează IP-ul LAN. |
| `ssh-unreachable` | Serverul nu poate fi atins prin SSH. | Verifică adresa și portul și dacă SSH este pornit. |
| `ssh-auth` | Serverul a refuzat cheia BombVault. | Rulează o dată pe server comanda arătată pe cardul conexiunii. |
| `zfs-not-found` | Gazda SSH nu are comanda `zfs`. | Îndreaptă Host SSH: Address spre mașina care deține pool-urile. |
| `zfs-permission` | Utilizatorul SSH nu are voie să ruleze această comandă zfs. | Folosește root sau vezi [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` numește altă gazdă sau alt utilizator decât câmpurile SSH. | Pune-le de acord sau golește câmpurile SSH, ca ambele să vină din URI. |
| `zfs-error` | zfs a raportat altă eroare. | Detaliile arată mesajul lui. |
| `propagation-missing` | Montările noi de pe gazdă nu ajung la container. | Setează Access Mode pentru Host Data la Read/Write - Slave și repornește BombVault. |
| `invalid-name` | Un nume de set de date pe care BombVault nu îl acceptă. | Redenumește setul de date. |
| `name-too-long` | Un set de date din arbore este prea lung pentru un nume de instantaneu. | Redenumește-l sau adaugă ca element un set de date de sub el. |
| `invalid-exclude` | Un model de excludere sau un copil exclus nu se potrivește cu elementul. | Corectează intrarea numită în mesaj. Ca să lași pe dinafară un set de date copil întreg, dezactivează-l în loc să scrii un model. |
| `not-found` | Setul de date nu există pe server. | Scoate elementul sau recreează setul de date. Copiile lui rămân restaurabile. |
| `not-filesystem` | Acesta e un volum, nu un sistem de fișiere. | Vezi [Volume](#volumes). |
| `overlaps-item` | Setul de date se suprapune cu un element existent. | Vezi [Elementele nu se suprapun niciodată](#overlap). |
| `docker-storage` | Arborele conține stocarea de imagini Docker. | Vezi [Stocarea Docker](#docker-storage). |
| `nothing-readable` | Niciun set de date al elementului nu poate fi citit acum. | Uită-te la codurile seturilor de date sărite. |
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

### Verifică ce vede containerul {#mountinfo}

**Testează accesul la instantanee** la un element face un instantaneu real al arborelui său, îl caută în BombVault pentru fiecare set de date și îl elimină din nou. E cel mai rapid mod de a dovedi tot drumul înainte de prima rulare programată.

Ca să te uiți singur, rulează asta pe server:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Fiecare linie este o montare în container. Linia unui set de date arată calea lui în container (sub `/host/user`) și numele setului de date. Un câmp `master:N` pe acea linie înseamnă că montarea primește montările pe care gazda le face mai târziu, iar de asta are nevoie accesul la instantanee. Dacă lipsește, setează Access Mode pentru Host Data la Read/Write - Slave și repornește BombVault.

## TrueNAS SCALE {#truenas}

- Când `LIBVIRT_URI` este setat (ca pentru backupurile VM pe TrueNAS), BombVault ia din URI gazda, utilizatorul și portul SSH pentru comenzile lui zfs, pe fiecare care nu e setat separat. Fără backupuri VM, setează în schimb `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` și `LIBVIRT_SSH_PORT`. Adaugă variabilele în **Additional Environment Variables**.
- Un alt utilizator decât root are nevoie de permisiuni pe setul de date de sus al elementului, care acoperă apoi fiecare set de date de sub el:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  O sesiune SSH fără root pe TrueNAS nu are `/usr/sbin` în cale; BombVault apelează atunci direct `/usr/sbin/zfs`.
- **Host Data** al aplicației trebuie să fie o cale a gazdei deasupra seturilor de date, de exemplu `/mnt/tank`, nu un ixVolume. Cu o cale a gazdei, aplicația transmite montările noi ale gazdei către BombVault (`rslave`), iar de asta are nevoie accesul la instantanee.
