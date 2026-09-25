# ZFS-datauppsättningar

Sidan **ZFS** säkerhetskopierar ZFS-datauppsättningar. Ett objekt är en datauppsättning tillsammans med alla datauppsättningar under den. För varje säkerhetskopia tar BombVault en enda ZFS-ögonblicksbild av hela trädet, så att varje datauppsättning i det fångas i samma ögonblick. Sedan läser det filerna i varje datauppsättning från den ögonblicksbilden, lagrar dem med restic på samma sätt som en mapp och tar bort ögonblicksbilden direkt efteråt. Säkerhetskopiorna är deduplicerade, du kan bläddra i var och en av dem, och enskilda filer kan återställas.

BombVault använder aldrig `zfs send` för datauppsättningar, rullar aldrig tillbaka en datauppsättning och förstör aldrig någon.

## Krav {#requirements}

- **SSH-anslutningen till den här servern.** ZFS-datauppsättningar använder samma nyckel, värd och användare som VM-säkerhetskopior. Fungerar VM-säkerhetskopior redan, fungerar det här också. Annars följer du [guiden för VM-säkerhetskopiering över SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) på GitHub. Mallfälten heter **Host SSH: Address**, **Host SSH: Port** och **Host SSH: User**.
- **Kommandot `zfs` på den värden.** Unraid 6.12 och senare samt TrueNAS SCALE har det.
- **Host Data mappad som `/mnt` med Access Mode Read/Write - Slave.** Det är mallens standard. Ögonblicksbilden av en datauppsättning dyker upp i datauppsättningens mapp `.zfs/snapshot` först efter att BombVault har startat, så containern måste ta emot monteringar som värden gör senare.
- **Datauppsättningarna monterade under `/mnt`.** På Unraid ligger pooler under `/mnt/<pool>`, så det är redan fallet.

Slå på domänen under **Inställningar, Allmänt** (ZFS-datauppsättningar). ZFS-sidan visar då kortet **Anslutning till den här servern**. Det testar SSH-anslutningen, anger användaren och värden det ansluter till och säger vad som saknas när något saknas. Kontrollen av värdintegrationen (`/spike`) visar samma resultat.

## Objekt och underliggande datauppsättningar {#items-and-children}

Öppna **Lägg till datauppsättningar** på ZFS-sidan. Listan kommer från servern. Välj den datauppsättning som ligger överst i det du vill säkerhetskopiera, till exempel `cache/appdata`, så omfattar objektet den och alla datauppsättningar under den.

- **Nya underliggande datauppsättningar kommer med av sig själva.** En datauppsättning som skapas senare under objektet säkerhetskopieras vid nästa körning, och den körningen anger den som ny. Dess första säkerhetskopia läser den helt en gång; därefter läses bara ändringar.
- **Du kan utelämna enskilda underliggande datauppsättningar.** Slå av en i objektets inställningar så lämnas den utanför tillsammans med allt under den. En utelämnad underliggande datauppsättning som inte längre finns på servern markeras som sådan och kan tas bort från listan.
- **Underliggande datauppsättningar som inte går att läsa hoppas över, aldrig i tysthet.** Körningen listar dem, objektet visar hur många som hoppades över och täckningskortet på översikten räknar var och en som oskyddad. Körningen säkerhetskopierar ändå allt annat och misslyckas inte på grund av en överhoppad datauppsättning. Orsakerna står i [tabellen över orsakskoder](#reason-codes): en datauppsättning som inte är monterad, har `canmount=off`, en `legacy`-monteringspunkt eller ingen alls, en krypteringsnyckel som inte är laddad, avstängd åtkomst till ögonblicksbilder eller en monteringspunkt som BombVault inte ser.
- **En överhoppad datauppsättning tar inte med sig sina underliggande.** En datauppsättning med `canmount=off` som bara innehåller andra datauppsättningar hoppas över (visas som "bara struktur"), och dess monterade underliggande datauppsättningar säkerhetskopieras. En krypterad datauppsättning vars nyckel inte är laddad hoppas över tillsammans med de underliggande som delar dess nyckel.
- **Underliggande datauppsättningar som är VM-diskar eller systemdata börjar avstängda** i dialogen för att lägga till, med orsaken bredvid omkopplaren. Lägger du till en hel pool får du bekräfta det i en dialog som listar vad den innehåller.

### Volymer {#volumes}

En volym (zvol) innehåller en virtuell disk i stället för filer, och ZFS-sidan säkerhetskopierar aldrig någon.

- En volym som en VM använder säkerhetskopieras med den VM:en på sidan **VMs**.
- En volym som ingen VM använder (en iSCSI-extent, en disk du har kopplat bort) **säkerhetskopieras inte av BombVault**. Dialogen för att lägga till och ZFS-sidan räknar de här volymerna och säger det. En senare version kommer att säkerhetskopiera dem.

Volymer i ett objekts träd hoppas över och nämns vid varje körning.

### Dockers lagring {#docker-storage}

Med Dockers ZFS-lagringsdrivrutin är varje avbildningslager en datauppsättning med `legacy`-monteringspunkt. Dialogen för att lägga till slår ihop dem till en rad per överordnad datauppsättning. Ett träd med fler än 20 sådana datauppsättningar kan inte bli ett objekt: så länge det finns en ögonblicksbild av det kan Docker inte ta bort avbildningslager. Lägg i stället till datauppsättningarna under det, till exempel `appdata`.

### Objekt överlappar aldrig {#overlap}

En datauppsättning kan bara höra till ett objekt. BombVault avvisar ett nytt objekt som ligger inuti ett befintligt eller som skulle innehålla ett. För att slå ihop flera underliggande objekt till ett överordnat tar du först bort de underliggande objekten och väljer att behålla deras säkerhetskopior, och lägger sedan till det överordnade. Varje datauppsättning behåller sin historik under sitt eget namn, så nästa säkerhetskopia fortsätter där de gamla objekten slutade och läser inte om allt.

## Stoppa containrar och köra kommandon runt ögonblicksbilden {#consistency}

En ögonblicksbild av en databas som körs är som ett plötsligt strömavbrott: databasen återhämtar sig oftast, men den måste göra det. Varje objekt kan göra två saker åt det, och båda gäller bara ögonblicket för ögonblicksbilden, inte hela säkerhetskopian.

- **Stoppa de här containrarna för ögonblicksbilden.** BombVault stoppar containrarna i listan, tar ögonblicksbilden och startar dem igen direkt. Containrar på samma beroendenivå stoppas parallellt, de beroende först, så hela fönstret varar oftast några sekunder; körningen visar hur länge. Säkerhetskopian läser sedan den frysta ögonblicksbilden medan apparna redan kör igen. Bara containrar som körde stoppas.
- **Ett kommando före och efter ögonblicksbilden.** Det körs i en container du väljer, till exempel för att dumpa en databas till datauppsättningen precis före ögonblicksbilden utan att stoppa något. Misslyckas kommandot före ögonblicksbilden misslyckas säkerhetskopian och ingen ögonblicksbild tas. Ett kommando efter ögonblicksbilden som misslyckas visas på körningen men får inte säkerhetskopian att misslyckas.

Vad som händer när något går fel:

- Kan en container inte stoppas startar BombVault de som redan har stoppats, och säkerhetskopian misslyckas med containerns namn. Det faller aldrig tillbaka på en ögonblicksbild av appar som körs.
- Stoppet väntar tills en pågående containersäkerhetskopia är klar (upp till 30 minuter vid en manuell körning, upp till säkerhetskopians tidsgräns vid en schemalagd), så att de två aldrig stoppar och startar samma container samtidigt.
- Innan den första containern stoppas skriver BombVault ner vilka det stoppar. Om BombVault dödas inom fönstret startar det de containrarna igen nästa gång det startar, skickar en avisering, och objektet visar en röd anteckning för varje container det inte kunde starta.

De automatiska databasdumparna (se [Funktioner](features.md)) körs med en containers egen säkerhetskopia på sidan **Containers**, inte med ett ZFS-objekt. En databas vars container bara säkerhetskopieras via sin datauppsättning får ingen dump, så ge den ett kommando här.

En container kan stå på den här listan och på sidan **Containers** samtidigt. Dess data lagras då två gånger, i två repositories, och **Total säkerhetskopia** stoppar den två gånger. Objektet påpekar det.

## Återställning {#restore}

Öppna **Säkerhetskopior** på objektet, välj säkerhetskopian och sedan datauppsättningen. Som standard är det objektets översta datauppsättning.

- **Återställ in i datauppsättningen.** Filer från säkerhetskopian skrivs till datauppsättningens monteringspunkt. Filer med samma namn skrivs över, andra ligger kvar. Datauppsättningen rullas aldrig tillbaka eller ersätts. BombVault kontrollerar att datauppsättningen är monterad, synlig och skrivbar, en gång innan det börjar och igen precis innan det skriver. Där en underliggande datauppsättning är monterad inuti skrivs ingenting: den behåller sina filer, sin ägare och sina behörigheter och återställs från sin egen säkerhetskopia.
- **Återställ till en mapp.** Välj en mapp under `/mnt`. BombVault kontrollerar att mappen ligger på en monterad pool eller resurs och att det finns tillräckligt med ledigt utrymme. Det fungerar utan SSH-anslutningen och för datauppsättningar som inte längre finns.
- **Välj filer** (avancerat): skriv bara tillbaka de filer och mappar du väljer till datauppsättningen.
- **Alla datauppsättningar i den här säkerhetskopian** (avancerat): varje datauppsättning i trädet i en egen undermapp i den mapp du väljer. Datauppsättningar som hoppades över i den säkerhetskopian nämns.
- **Från en annan server:** sidan **Återställning** återställer från en annan BombVaults repository, alltid till en mapp: alla datauppsättningar i en säkerhetskopia, var och en i en egen undermapp, eller en datauppsättning i trädet, hel eller valda filer.

Objektets lista över containrar som ska stoppas erbjuds även vid återställning in i datauppsättningen. De containrarna förblir stoppade under hela återställningen, och containersäkerhetskopior väntar så länge.

### Säkerhetsögonblicksbilden {#safety-snapshot}

Innan det skriver till en datauppsättning tar BombVault en ZFS-ögonblicksbild av just den datauppsättningen, med namnet `bombvault-prerestore-<tid>`. Den är på som standard; att stänga av den kräver en andra bekräftelse. Om ögonblicksbilden inte kan tas återställs ingenting.

BombVault tar aldrig bort en säkerhetsögonblicksbild av sig självt. Objektet listar dem med ålder och storlek, var och en med åtgärden **Ta bort**, och varnar när den äldsta är mer än 30 dagar gammal, eftersom den håller kvar raderad och ändrad data i poolen.

För att gå tillbaka efter en återställning kopierar du enskilda filer från `.zfs/snapshot/bombvault-prerestore-<tid>` inuti datauppsättningen. `zfs rollback <dataset>@bombvault-prerestore-<tid>` fungerar bara så länge den är datauppsättningens senaste ögonblicksbild. `zfs rollback -r` tar bort alla nyare ögonblicksbilder, även automatiska.

### Återställa som ny datauppsättning {#new-dataset}

BombVault skapar inte datauppsättningar. Skapa den på servern med de egenskaper du vill ha och återställ sedan till en mapp som är dess monteringspunkt:

```
zfs create -o compression=lz4 cache/appdata-restored
```

och återställ i BombVault till mappen `cache/appdata-restored` under `/mnt`.

## Vad säkerhetskopian innehåller {#contents}

I säkerhetskopian: filerna och mapparna i varje säkerhetskopierad datauppsättning, med ägare, behörigheter, tidsstämplar och utökade attribut så som restic lagrar dem.

Inte i säkerhetskopian:

- datauppsättningarnas ZFS-egenskaper (compression, recordsize, quota, mountpoint och resten);
- ägare och behörigheter för själva den översta mappen i varje datauppsättning (allt under den ingår). En återställning in i datauppsättningen lämnar den befintliga översta mappen som den är, en återställning till en mapp skapar den med behörigheterna `0755`;
- befintliga ZFS-ögonblicksbilder;
- underliggande datauppsättningar som hoppades över eller utelämnades;
- volymer.

För att återställa till en ny pool skapar du först datauppsättningarna med de egenskaper du vill ha. Om NFSv4-ACL:er, så som TrueNAS använder dem på SMB-datauppsättningar, kommer tillbaka som du förväntar dig är ännu inte kontrollerat, så testa en återställning med dina egna data innan du litar på dem.

## Krypterade datauppsättningar {#encryption}

En krypterad datauppsättning säkerhetskopieras bara medan dess nyckel är laddad. Annars hoppas den över med en varning; ladda nyckeln med `zfs load-key` och montera datauppsättningen. BombVault läser data dekrypterat och lagrar det i restics repository, som är krypterat. Har du stängt av kryptering i BombVault är det repositoryt inte krypterat.

## Kvarlämnade ögonblicksbilder {#leftover-snapshots}

Ögonblicksbilden av en säkerhetskopia heter `<dataset>@bombvault-<14 siffror>`, till exempel `cache/appdata@bombvault-20260924021500` (UTC). BombVault tar bort den direkt efter säkerhetskopian. Om det misslyckas, till exempel för att datauppsättningen är upptagen eller BombVault stoppades, tar BombVault bort den:

- före nästa säkerhetskopia av det objektet,
- när BombVault startar, för varje objekt, även med domänen avstängd,
- när du tar bort objektet,
- när du trycker på **Ta bort nu** på objektet, som också visar hur många som är kvar.

Bara namn som exakt är `bombvault-` plus 14 siffror tas bort. Säkerhetsögonblicksbilder, dina egna ögonblicksbilder och automatiska ögonblicksbilder rörs aldrig. För att ta bort en för hand:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

## Avvikelser {#anomalies}

En underliggande datauppsättning som har tömts ändrar knappt summan för ett stort träd, så avvikelsedetekteringen bevakar varje datauppsättning i ett objekt för sig: storlek, antal filer, ny data och restic-tid har var sin egen historik. Den historiken hör till datauppsättningens namn, så den finns kvar när trädet senare säkerhetskopieras av ett annat objekt.

En datauppsättning som föregående körning säkerhetskopierade och som den här körningen inte kunde läsa räknas som tömd, så länge objektets urval inte har ändrats. Det täcker en nyckel som inte är laddad, en datauppsättning som inte är monterad och en som har försvunnit ur trädet. En underliggande datauppsättning som du själv utesluter ändrar urvalet, så dess historik börjar om i stället. Så länge ett fynd om förlorad data är öppet behåller lagringen de gamla säkerhetskopiorna av just den datauppsättningen och rensar resten av trädet som vanligt.

På fliken **Objekt** på sidan **Avvikelser** har varje datauppsättning en egen rad under sitt objekt, och objektets träd på den här sidan visar de öppna fynden bredvid varje datauppsättning. Länken i ett fynd öppnar objektets återställningspanel vid datauppsättningens senaste goda säkerhetskopia. Om en körning blir klar bedöms för hela objektet, eftersom en körning lyckas eller misslyckas som helhet.

Själva kontrollerna beskrivs under [Funktioner](features.md). En assistent som är ansluten via [MCP-servern](mcp.md) kan lista ett ZFS-objekts återställningspunkter, starta dess säkerhetskopia och läsa fynden, men ett fynd kvitteras på sidan **Avvikelser**.

## Orsakskoder {#reason-codes}

Sidan, körningshistoriken och aviseringarna anger ett problem med en av de här koderna. De flesta har också lösningen bredvid på sidan.

| Kod | Betydelse | Vad du gör |
|---|---|---|
| `ssh-missing` | SSH-anslutningen är inte konfigurerad i den här containern. | Konfigurera SSH-anslutningen som för VM-säkerhetskopior. |
| `host-placeholder` | Host SSH: Address är fortfarande exempelvärdet, och `host.docker.internal` svarade inte heller. | Sätt Host SSH: Address till den här serverns LAN-IP. |
| `host-fallback` | Host SSH: Address är fortfarande exempelvärdet, och `host.docker.internal` fungerar. | Inget, eller sätt LAN-IP:n. |
| `ssh-unreachable` | Servern går inte att nå över SSH. | Kontrollera adress och port, och att SSH är påslaget. |
| `ssh-auth` | Servern avvisade BombVaults nyckel. | Kör kommandot från anslutningskortet en gång på servern. |
| `zfs-not-found` | SSH-värden har inget `zfs`-kommando. | Peka Host SSH: Address mot maskinen som äger poolerna. |
| `zfs-permission` | SSH-användaren får inte köra det här zfs-kommandot. | Använd root, eller se [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` anger en annan värd eller användare än SSH-fälten. | Få dem att stämma, eller töm SSH-fälten så att båda kommer från URI:n. |
| `zfs-error` | zfs rapporterade ett annat fel. | Detaljerna visar dess meddelande. |
| `propagation-missing` | Nya monteringar på värden når inte containern. | Sätt Access Mode för Host Data till Read/Write - Slave och starta om BombVault. |
| `invalid-name` | Ett namn på en datauppsättning som BombVault inte godtar. | Byt namn på datauppsättningen. |
| `name-too-long` | En datauppsättning i trädet är för lång för ett namn på en ögonblicksbild. | Byt namn på den, eller lägg till en datauppsättning under den som objekt. |
| `invalid-exclude` | Ett uteslutningsmönster eller en utelämnad underliggande datauppsättning passar inte objektet. | Rätta posten som meddelandet nämner. För att utelämna en hel underliggande datauppsättning stänger du av den i stället för att skriva ett mönster. |
| `not-found` | Datauppsättningen finns inte på servern. | Ta bort objektet eller skapa datauppsättningen igen. Dess säkerhetskopior går fortfarande att återställa. |
| `not-filesystem` | Det här är en volym, inte ett filsystem. | Se [Volymer](#volumes). |
| `overlaps-item` | Datauppsättningen överlappar ett befintligt objekt. | Se [Objekt överlappar aldrig](#overlap). |
| `docker-storage` | Trädet innehåller Dockers avbildningslagring. | Se [Dockers lagring](#docker-storage). |
| `nothing-readable` | Ingen datauppsättning i objektet går att läsa just nu. | Titta på koderna för de överhoppade datauppsättningarna. |
| `snapshot-failed` | Ögonblicksbilden kunde inte skapas. | Detaljerna visar meddelandet från zfs. |
| `containers-busy` | En containersäkerhetskopia körde fortfarande när containrarna skulle stoppas. | Starta igen senare. Schemalagda körningar väntar av sig själva. |
| `consistency-stop-failed` | En container kunde inte stoppas, så ingen ögonblicksbild togs. | Kontrollera containern, eller ta bort den från listan. |
| `pre-snapshot-failed` | Kommandot före ögonblicksbilden misslyckades. | Körningsdetaljerna visar dess utdata. |
| `container-unknown` | En container i listan finns inte. | Ta bort den från listan. |
| `container-is-self` | BombVault kan inte stoppa sin egen container. | Ta bort den från listan. |
| `leftover-snapshots` | Ögonblicksbilder som BombVault inte kunde ta bort ligger kvar på servern. | Tryck på **Ta bort nu**, se [Kvarlämnade ögonblicksbilder](#leftover-snapshots). |
| `zvol` | En volym i trädet, överhoppad. | Se [Volymer](#volumes). |
| `canmount-off` | Aldrig monterad (`canmount=off`), överhoppad. | Innehåller den data, montera den eller flytta datan till en underliggande datauppsättning. |
| `legacy-mount` | Legacy-monteringspunkt, överhoppad. | Ge den en monteringspunkt under `/mnt`. |
| `no-mountpoint` | Ingen monteringspunkt, överhoppad. | Ge den en monteringspunkt under `/mnt`. |
| `not-mounted` | Inte monterad på servern, överhoppad. | Montera den med `zfs mount`, eller sätt `canmount=on`. |
| `key-not-loaded` | Krypterad och nyckeln är inte laddad, överhoppad. | `zfs load-key`, montera den sedan. |
| `snapdir-disabled` | Åtkomst till ögonblicksbilder är avstängd, överhoppad. | `zfs set snapdir=hidden <dataset>`. Mappen `.zfs` förblir dold. |
| `not-visible` | BombVault ser inte datauppsättningens monteringspunkt. | Flytta monteringspunkten under Host Data-sökvägen, eller mappa in den i containern på samma sökväg med Read/Write - Slave. |
| `shfs-only` | Datauppsättningen syns bara via `/mnt/user`, som döljer ögonblicksbilder. | Mappa `/mnt`, inte `/mnt/user`, som Host Data. |
| `snapshot-not-visible` | Ögonblicksbilden skapades men dök inte upp i BombVault. | Kör **Testa åtkomsten till ögonblicksbilder**; se nedan. |
| `snapshot-loop` | Ögonblicksbilden nådde inte BombVault eftersom Host Data inte släpper igenom nya monteringar. | Sätt Access Mode för Host Data till Read/Write - Slave och starta om BombVault. |
| `backup-failed` | restic misslyckades för den här datauppsättningen. | Körningsdetaljerna visar varför. |
| `not-reached` | Körningen slutade före den här datauppsättningen. | Kör säkerhetskopian igen. |
| `gone` | Datauppsättningen finns inte längre på servern. | Inget. Dess säkerhetskopior går fortfarande att återställa. |
| `read-only-mount` | BombVault kan bara läsa datauppsättningen och kan därför inte återställa in i den. | Sätt mappningen till Read/Write - Slave, eller återställ till en mapp. |
| `destination-not-mounted` | Mappen ligger inte på en monterad pool eller resurs. | Välj en mapp på en pool eller resurs. |
| `not-enough-space` | Inte tillräckligt med ledigt utrymme på målet. | Frigör utrymme eller välj en annan mapp. |
| `safety-snapshot-failed` | Säkerhetsögonblicksbilden kunde inte tas, så ingenting återställdes. | Detaljerna visar meddelandet från zfs. |
| `safety-name-too-long` | Namnet på datauppsättningen är för långt för en säkerhetsögonblicksbild. | Stäng av säkerhetsögonblicksbilden, eller återställ till en mapp. |

### Kontrollera vad containern ser {#mountinfo}

**Testa åtkomsten till ögonblicksbilder** på ett objekt tar en riktig ögonblicksbild av dess träd, letar efter den i BombVault för varje datauppsättning och tar bort den igen. Det är det snabbaste sättet att bevisa hela vägen före den första schemalagda körningen.

För att titta själv kör du det här på servern:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Varje rad är en montering i containern. Raden för en datauppsättning visar dess sökväg i containern (under `/host/user`) och namnet på datauppsättningen. Ett fält `master:N` på den raden betyder att monteringen tar emot de monteringar som värden gör senare, och det är vad åtkomst till ögonblicksbilder behöver. Saknas det sätter du Access Mode för Host Data till Read/Write - Slave och startar om BombVault.

## TrueNAS SCALE {#truenas}

- När `LIBVIRT_URI` är satt (som för VM-säkerhetskopior på TrueNAS) tar BombVault SSH-värd, användare och port för sina zfs-kommandon från URI:n, var och en som inte är satt för sig. Utan VM-säkerhetskopior sätter du i stället `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` och `LIBVIRT_SSH_PORT`. Lägg till variablerna under **Additional Environment Variables**.
- En annan användare än root behöver behörigheter på objektets översta datauppsättning, som då täcker alla datauppsättningar under den:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  En SSH-session utan root på TrueNAS har inte `/usr/sbin` i sin sökväg; BombVault anropar då `/usr/sbin/zfs` direkt.
- Appens **Host Data** måste vara en värdsökväg ovanför datauppsättningarna, till exempel `/mnt/tank`, inte en ixVolume. Med en värdsökväg skickar appen värdens nya monteringar vidare till BombVault (`rslave`), och det behöver åtkomst till ögonblicksbilder.
