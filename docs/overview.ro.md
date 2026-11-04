# Prezentare

Prezentarea este prima pagină din bara laterală și cea de verificat în fiecare dimineață. Răspunde dintr-o privire la două întrebări: este totul salvat și are nevoie ceva de tine? Începe cu un antet de stare și patru carduri, iar alte carduri așteaptă sub **Personalizează**.

![Prezentarea după câteva zile: antetul de stare, Necesită atenție cu un avertisment, protecția fiecărei secțiuni, destinațiile și istoricul.](assets/screenshots/overview/home-dark.png#only-dark)
![Prezentarea după câteva zile: antetul de stare, Necesită atenție cu un avertisment, protecția fiecărei secțiuni, destinațiile și istoricul.](assets/screenshots/overview/home-light.png#only-light)

## Antetul de stare {#status-head}

Antetul stă sus în pagină și rămâne mereu acolo. Inelul lui arată cât este protejat, de exemplu **41 din 43 protejate**, iar rândul de lângă el spune cum stau lucrurile:

| Antetul spune | Ce înseamnă |
|---|---|
| **Totul este salvat** | Fiecare element din fiecare secțiune pornită are un backup curent. |
| **Salvat, cu observații** | Totul este salvat, iar unele rulări au lăsat o notă, de exemplu fișiere pe care nu le-au putut citi. |
| **Nu totul este protejat încă** | Unele elemente nu au un backup curent, de exemplu pentru că așteaptă primul. |
| **3 lucruri au nevoie de tine** | Ceva a eșuat sau așteaptă o decizie. Cardul **Necesită atenție** le listează. |
| **Se face backup acum** | Rulează un backup. Inelul devine un inel de progres cu stațiile **Oprire**, **Citire** și **Pornire**, iar un backup total își numără elementele pe parcurs, de exemplu **Se face backup acum: 12 din 43**. **Vezi rularea** deschide rularea. |
| **Copierea off-site rulează** | Rulează o copiere către o destinație. Nu arată procent, pentru că `restic copy` nu raportează unul. |

Dedesubt apare când a fost ultima rulare și când pornește următoarea planificată.

## Necesită atenție {#needs-you}

Acest card listează ce ar trebui să verifici, cele mai noi primele, și spune **Nimic de făcut.** când lista este goală. Fiecare rând are propria ieșire:

- **Un backup eșuat** numește elementul și motivul. **Vezi erorile** arată ieșirea proprie a lui restic, iar **Încearcă din nou** pornește backupul încă o dată.
- **Copierea off-site a eșuat** numește secțiunea și de când. **Replică acum** încearcă din nou.
- **Anomalie la plex** duce la constatare cu **Verifică**. Cât timp o constatare critică este deschisă, retenția păstrează backupurile vechi ale acelui element.
- Elementele **Noi** pe care BombVault le-a găsit și pe care încă nu le salvează nimic, de exemplu un container tocmai instalat, oferă **Începe backup-ul** sau **Fără copie de rezervă**.
- **Un backup care a renunțat** pentru că aplicația lui a rămas ocupată pe toată durata așteptării, vezi [Când aplicația este ocupată](backups.md#app-busy).
- **O replică ZFS eșuată**, în cuvinte simple, de exemplu că serverul nu are comanda zfs, cu ieșirea serverului la **Vezi**.
- **Copiile de rezervă sunt oprite: cheia nu se mai poate deschide** apare când acest server a preluat cheia altui server, iar `APP_KEY` s-a schimbat după aceea. Vezi [Depanare](troubleshooting.md#backups-stopped-and-say-the-key-can-no-longer-be-opened).
- **Salvează-ți kitul de recuperare** rămâne până când descarci kitul și apeși **L-am păstrat în siguranță**.

## Starea protecției {#protection-status}

Fiecare secțiune pornită are un rând cu patru verificări:

| Verificare | Devine verde când | Alte cuvinte pe care le poți vedea |
|---|---|---|
| **Local** | ultimul backup este curent | încă niciun backup, fără programare, ultima rulare a eșuat, backup întârziat, nu fiecare element este protejat, backup scadent curând |
| **Extern** | ultima copie din afara locației este curentă | fără copie off-site, copiere în curs, copie întârziată, copie în pauză, încă nicio copie, replică eșuată |
| **Protecție împotriva ștergerii** | un test de manipulare a dovedit că partea îndepărtată refuză ștergerile | niciuna, niciuna și risc acceptat, încă nedovedită, testul a eșuat, nedovedită pentru că verificarea ei este oprită |
| **Restaurare de test** | o restaurare din copia off-site a reușit | niciodată rulată, rulează, eșuată, întârziată, doar manual, imposibilă pentru această secțiune |

O secțiune este verde doar pe fapte verificate. O copie pe acest server nu contează ca off-site, iar o destinație marcată **Acceptă fără protecție împotriva ștergerii** arată un inel gri în loc de un punct chihlimbariu, așa că culoarea secțiunii urmează celelalte verificări ale ei.

Un clic pe o secțiune deschide fereastra ei cu fiecare verificare explicată, elementele pe care le numără și **Pornește restaurarea de test** sau **Rulează din nou restaurarea de test**.

![Fereastra de protecție a secțiunii Containere cu cele patru verificări explicate.](assets/screenshots/overview/protection-window-dark.png#only-dark)
![Fereastra de protecție a secțiunii Containere cu cele patru verificări explicate.](assets/screenshots/overview/protection-window-light.png#only-light)

## Destinații {#storage-locations}

Fiecare destinație apare aici ca un vas cu nivelul de umplere, cât de repede crește, de exemplu **În creștere cu 4,2 GB/săptămână**, și când ar fi plină la acest ritm, de exemplu **plină în aproximativ 31 de săptămâni**. S3, B2 și serverele rest nu raportează cât loc a rămas, așa că arată **Spațiu liber necunoscut** și doar creșterea. Un clic duce la pagina destinației. Vezi [Destinații](offsite-recovery.md#destinations).

## Istoric {#history}

**Istoric** are două file. **Rulări** listează fiecare rulare cu rezultatul, durata și motivul ei, iar un clic o deschide. **Jurnal** arată fiecare rând cu ora lui, așa cum îl scrie BombVault, și poate fi restrâns la o zi. Un rând devine portocaliu când un backup a trecut, dar a lăsat ceva pe dinafară, iar rândul spune ce a lăsat.

## Mai multe carduri sub Personalizează {#customize}

**Personalizează**, sus în pagină, te lasă să aranjezi cardurile. Trage un card de bara lui ca să-l muți, redimensionează-l de punctul din colțul de jos sau ascunde-l. Cardurile ascunse așteaptă sub **Carduri ascunse**, iar **Afișează** readuce unul. Cât le aranjezi, cardurile se leagănă puțin, iar **Gata** încheie. Aranjamentul se salvează în acest browser.

Aceste carduri încep ascunse:

- **Anomalii**: constatările deschise, vezi [Anomalii](anomalies.md).
- **Fără backup**: tot ce se află pe acest server și nu este acoperit de niciun backup programat, fiecare cu motivul lui, de exemplu **niciodată adăugat**, **nu este inclus în programare** sau **toate destinațiile secțiunii sale sunt oprite**. Citește containerele direct din Docker, așa că găsește și unul pe care BombVault nu l-a văzut niciodată.
- **Calendar**: ultimele 90 de zile, un pătrat pe zi. Un clic pe o zi arată rândurile ei din jurnal la **Istoric**.
- **Stocare**: cât ocupă fiecare secțiune și cât de bine se deduplică datele ei.
- **Prezentare**: cel mai vechi instantaneu din toate secțiunile, când rulează următorul backup și cum a mers ultimul.
- **Statistici**: câte containere și VM-uri există, câte erori și câte planuri sunt active sau în pauză.
- **Integrare gazdă**: ce atinge și de ce are nevoie BombVault pe gazdă, de exemplu socketul Docker, libvirt, restic și rclone. Începe de aici când ceva pare nelegat.

![Modul Personalizează: cardurile pot fi trase, redimensionate și ascunse, iar cele ascunse așteaptă dedesubt.](assets/screenshots/overview/customize-dark.png#only-dark)
![Modul Personalizează: cardurile pot fi trase, redimensionate și ascunse, iar cele ascunse așteaptă dedesubt.](assets/screenshots/overview/customize-light.png#only-light)

## Cum numără BombVault {#counting}

Fiecare card și antetul numără la fel: elementele secțiunilor pornite care sunt încă instalate și pe care nu le-ai marcat **Fără copie de rezervă**. Un element este socotit protejat doar cât timp o programare îl atinge și are un backup. Un backup vechi singur nu contează, așa că un container scos din programare apare ca neprotejat până îl pui la loc sau îl marchezi **Fără copie de rezervă**.

## Primii pași {#first-steps}

La o instalare nouă, prezentarea începe cu cardul **Primii pași**: **Alege secțiunile**, **Adaugă destinația** și **Prima copie de rezervă**. Rămâne până se termină primul backup. [Introducere](getting-started.md#first-run) te conduce prin el.
