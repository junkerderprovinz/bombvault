# Programări

**Setări, Programare** conține toate planurile: backupul total care acoperă tot, un plan propriu pentru o secțiune, copiile off-site, ce elemente participă și în ce ordine. Un singur element poate primi un plan propriu pe pagina lui. Această pagină explică cum se îmbină și parcurge o configurare tipică.

## Planurile se citesc ca propoziții {#plans}

Fiecare plan este un buton care se citește ca o propoziție, de exemplu *zilnic la 03:00* sau *la fiecare 2 zile la 01:30*. Un clic îl deschide cu cinci moduri:

| Mod | Ce setezi |
|---|---|
| **Oprit** | Nimic nu rulează pe acest plan. |
| **Zilnic** | **Oră** |
| **Săptămânal** | **Zile** și **Oră** |
| **La fiecare N zile** | **La fiecare** atâtea **zile**, plus **Oră**. Numără de la ultima rulare programată, așa că un backup făcut manual pentru un element nu împinge planul înapoi. |
| **Cron** | O **Expresie** cu cinci câmpuri, de exemplu `0 */6 * * *` pentru la fiecare 6 ore. **Exemple** oferă câteva, iar fereastra arată următoarele rulări. |

Toate orele sunt în fusul orar al serverului. Pe Unraid este cel al sistemului. În altă parte setează `TZ`, altfel fiecare plan rulează în UTC, vezi [Variabile de mediu](configuration.md#environment-variables).

## Săptămâna {#week}

Cardul **Săptămâna** desenează fiecare rulare planificată de luni până duminică, câte o linie pe zi: backupuri, copii off-site și verificări. Treci cu mouse-ul peste o liniuță ca să citești ce pornește acolo. Unde trei planuri pornesc într-un interval de două ore, porțiunea este marcată, ca să le poți răspândi.

![Setări, Programare: banda Săptămâna sus, apoi backupul total și planurile pe secțiuni.](assets/screenshots/schedule/week-dark.png#only-dark)
![Setări, Programare: banda Săptămâna sus, apoi backupul total și planurile pe secțiuni.](assets/screenshots/schedule/week-light.png#only-light)

## Backup total {#full-backup}

**Backup total** este planul implicit pentru tot. Îl setează **Când rulează backupurile**: o singură trecere care face backup fiecărei secțiuni pe rând, containere, VM-uri, flash, foldere, seturi de date ZFS și, la sfârșit, backupul propriu. Rândul de dedesubt spune ce secțiuni include. **Oprit** îl dezactivează, iar atunci rulează doar ce are un plan propriu.

- **Recuperează backupurile ratate după pornire**: dacă serverul era oprit la o oră planificată, BombVault rulează acel backup la aproximativ două minute după ce pornește. Planul propriu al unui element și un plan off-site așteaptă următoarea lor oră planificată, la fel și un plan care nu a reușit niciodată.
- **Comenzi înainte și după**: **Comandă înainte de backupul total** rulează pe server cu `sh -c` înainte să înceapă trecerea, de exemplu un script care oprește un serviciu. Dacă eșuează, backupul total rulează oricum. **Comandă după backupul total** rulează o singură dată după ce fiecare secțiune a fost încercată, chiar dacă una a eșuat, ceea ce se potrivește cu un ping care confirmă că întregul server este protejat. Comenzile pentru un singur container se află pe pagina lui.

**Copiază tot acum** din pagina Copii de rezervă pornește manual aceeași trecere, peste fiecare secțiune pornită.

## Planuri pe secțiuni {#plans-per-section}

O secțiune poate părăsi backupul total și rula la ora ei. În **Programări pe secțiuni**, **Excepție** adaugă o secțiune cu plan propriu, iar **Elimină excepția** o trimite înapoi. Rândul din card spune ce fac celelalte, de exemplu **Toate celelalte secțiuni rulează în backupul total.**

O secțiune cu plan propriu nu mai este salvată o dată în backupul total, așa că nimic nu este salvat de două ori. Același lucru este valabil pentru un singur element cu plan propriu.

**Copii externe pe secțiuni** funcționează la fel pentru copii. Implicit fiecare secțiune copiază în destinațiile ei off-site imediat după fiecare backup. **Excepție** dă unei secțiuni un plan propriu de copiere, de exemplu ca VM-urile să se copieze pe Backblaze B2 doar duminica noaptea. Cardul **Copiază** al unei destinații poate seta și **Când se copiază**, vezi [Copii off-site](offsite-recovery.md#off-site-copies).

## Elemente programate {#scheduled-entries}

**Elemente programate** spune pentru fiecare secțiune câte elemente participă, de exemplu **38 din 41 programate**. **Elemente** deschide lista cu un comutator **În programare** pentru fiecare element. Un element oprit este salvat doar când îl pornești manual. Este socotit neprotejat, dacă nu îl marchezi **Fără copie de rezervă**.

## Ordine {#order}

**Ordine** stabilește ordinea în care rulările programate și backupul total fac backup containerelor și VM-urilor. Trage un element la locul lui sau folosește săgețile, iar fiecare mutare se salvează imediat. Elementele pe care le lași fără ordine sunt salvate începând cu cel mai întârziat. Un singur backup manual nu este afectat.

**Repornire după backup** din același card se ocupă de containerele pe care **Oprește alte containere** le-a oprit în timpul unui backup. Ele pornesc din nou în ordinea dependențelor lor, iar cu **Așteaptă ca dependențele să fie sănătoase înainte de a-l porni pe următorul** fiecare trebuie să raporteze că este sănătos sau pornit înainte ca următorul să înceapă. **Timp de așteptare pentru sănătate per container (secunde)** limitează așteptarea, implicit 120, într-un interval de la 5 la 3600.

## Instantaneu în loc de oprire {#snapshot-instead-of-stop}

**Oprește containerele pe ZFS doar pentru un instantaneu** este pornit implicit. Un container ale cărui date se află pe ZFS este oprit doar cât BombVault face un instantaneu al acelor date, de obicei câteva secunde, iar backupul este citit apoi din instantaneu. Are nevoie de conexiunea SSH către gazdă pe care o folosesc seturile de date ZFS. Un container cu o parte din date în altă parte este oprit pe toată durata backupului, ca înainte, iar rularea lui spune de ce, în timp ce un container fără date pe ZFS este oprit fără nicio notă. Funcționează pe un share de utilizator Unraid aflat pe un singur pool ZFS. Pe TrueNAS aplicația are nevoie și de capabilitatea `SYS_CHROOT`, vezi [TrueNAS SCALE](getting-started.md#truenas). Un singur container poate fi setat altfel pe propria pagină.

Backupul ajunge sub aceleași căi și etichete ca unul făcut cu containerul oprit, așa că restaurările și verificările le tratează la fel. Comanda dinaintea backupului și dumpul bazei de date rulează tot primele, celelalte containere de oprit se opresc și revin odată cu el, iar comanda de după backup rulează de îndată ce containerul este din nou pornit. O actualizare după backup tot așteaptă până se termină backupul. Rulările lui arată **Instantaneu** și câte secunde a fost oprit containerul. Dacă BombVault se oprește în mijlocul oricărui backup de container, repornește data viitoare când pornește containerele pe care le ținea oprite.

## Actualizări după backup {#updates}

- **Actualizează containerele după o copie de rezervă reușită** trage o imagine mai nouă abia după ce backupul dinaintea ei s-a terminat, așa că există mereu mai întâi un punct de restaurare proaspăt. Este oprit implicit, iar un singur container poate fi setat altfel pe pagina lui, la **Actualizează după o copie de rezervă reușită**. Doar ce ai setat tu trece pe imaginea nouă: variabilele, etichetele, comanda, utilizatorul și healthcheckul care arată încă la fel ca în imaginea veche o urmează pe cea nouă. Dacă imaginea nouă nu poate fi creată sau pornită, containerul revine la imaginea pe care rula, actualizarea apare ca eșuată, iar următorul backup încearcă din nou. Un container rămas oprit din cauza asta este numit în rularea backupului și într-o notificare.
- **Elimină imaginea veche după o actualizare** șterge imaginea pe care containerul a părăsit-o. O imagine de bază pe care o împart alte containere nu este ștearsă niciodată.
- **Reîmprospătează starea actualizărilor Unraid după actualizarea unui container** șterge bannerul de actualizare al Unraid pentru acel container.

Un mesaj pentru fiecare container actualizat se setează la **Setări, Notificări** cu **Notifică la actualizarea containerului**.

## Inactivitate înainte de backup {#idle}

Cardul **Inactivitate înainte de copie** stabilește când o aplicație este socotită inactivă pentru un container al cărui backup programat o așteaptă: **CPU sub (% dintr-un nucleu)**, **Trafic sub (Mbit/s)** și **Cel puțin (min)**. Un server media este inactiv când nu face streaming, iar pentru un container pe rețeaua gazdei contează doar CPU. Așteptarea în sine se setează pe fiecare container, vezi [Când aplicația este ocupată](backups.md#app-busy).

## Un plan propriu pentru un element {#own-plan}

Fiecare element are **Programare proprie** în cardul **Programare și destinație** din fila Setări a lui. Cu un plan acolo, elementul rulează la orele lui și părăsește backupul total și planul secțiunii sale. Fără plan, le urmează.

## Exemplu: trei planuri care se potrivesc {#example}

O stație Unraid tipică: majoritatea containerelor pot fi salvate noaptea, VM-urile sunt mari și se schimbă încet, iar Immich primește poze noi toată ziua.

1. **Backup total la 03:00.** Deschide **Setări, Programare**. La **Backup total**, setează **Când rulează backupurile** pe **Zilnic** la `03:00`. Pornește **Recuperează backupurile ratate după pornire**, ca o noapte cu serverul oprit să fie recuperată.
2. **VM-uri o dată pe săptămână.** La **Programări pe secțiuni**, apasă **Excepție**, alege **VM-uri** și setează planul pe **Săptămânal**, **Zile** duminică, **Oră** `04:00`. Cardul spune acum că toate celelalte secțiuni rulează în backupul total.
3. **VM-urile se copiază off-site tot duminică.** La **Copii externe pe secțiuni**, apasă **Excepție**, alege **VM-uri** și setează **Săptămânal**, duminică la `06:00`, după ce backupul lor a avut timp să se termine.
4. **Immich la fiecare 6 ore.** Deschide **Copii de rezervă**, apoi `immich`, apoi fila **Setări**. Setează **Programare proprie** pe **Cron** cu `0 */6 * * *`.

Ce rulează acum:

| Când | Ce |
|---|---|
| În fiecare zi la 03:00 | Containerele fără `immich`, flash, foldere, seturi de date ZFS și backupul propriu, o secțiune după alta |
| Duminică la 04:00 | VM-uri |
| Duminică la 06:00 | Copia off-site a VM-urilor |
| 00:00, 06:00, 12:00 și 18:00 | `immich` separat |

Cardul **Săptămâna** arată linia de la 03:00 în fiecare zi, liniuțele de la 04:00 și 06:00 duminica și patru liniuțe pe zi pentru `immich`. `immich` nu este salvat a doua oară la 03:00, pentru că un element cu plan propriu părăsește backupul total. Copiile celorlalte secțiuni rulează în continuare imediat după fiecare backup.

Când elimini mai târziu planul propriu al lui `immich`, acesta se alătură din nou rulării de la 03:00 împreună cu secțiunea lui.
