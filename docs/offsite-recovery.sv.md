# Off-site och återställning

Lokala säkerhetskopior skyddar dig mot en förlorad container eller en dålig uppdatering. Off-site-replikering och ett testat återställningskit skyddar dig mot hela boxen, ransomware eller en brand. Den här sidan täcker att replikera off-site, att göra den kopian manipuleringssäker, att bevisa att du kan återställa, och att återhämta dig när BombVault självt är borta.

## Off-site-replikering

Behåll den snabba lokala säkerhetskopian och kopiera den till en eller flera andra lagringsplatser. Du väljer vilka lagringsplatser en domän kopieras till på kortet **Domäner** under **Inställningar, Lagring**, ett chip per lagringsplats (se [Lagringsplatser](storage-places.md#domains)). BombVault kopierar nya ögonblicksbilder dit med `restic copy` på best-effort-basis, så att en misslyckad kopia aldrig får den lokala säkerhetskopian att misslyckas. Lagringsplatsen som en domän sparas på behöver inte vara lokal; se [En domän sparad på en fjärransluten lagringsplats](#remote-primary-repositories).

- **Flera kopieringsplatser per domän.** En domän kan kopieras till flera lagringsplatser samtidigt, till exempel en rest-server hemma hos en vän och en B2-bucket. Retention, lagringsklass, append-only, gränser och tillväxtbudget hör till lagringsplatsen, så varje kopia följer reglerna för den lagringsplats den hamnar på.
- **Kopieringsschema per domän** (redigerat tillsammans med alla andra scheman under Inställningar, Scheman): lämna det tomt för att kopiera efter varje lokal säkerhetskopiering, eller sätt en kadens (till exempel `weekly Sun 03:00`) för att kopiera mer sällan än du säkerhetskopierar. **Kopiera nu** på domänens rad kör det på begäran.
- **Retention per lagringsplats.** Varje lagringsplats har sina egna regler, så en off-site-lagringsplats kan behålla kopior längre som ett arkiv. En lagringsplats där varje regel står på noll trimmar aldrig.
- **Bandbreddsgränser** per lagringsplats begränsar restics uppladdnings- och nedladdningshastighet så att kopieringen inte mättar din WAN.
- En **replikeringsindikator** visar vilken domän som kopieras medan det pågår (på dess sida och Översikten). Det är en aktiv indikator, inte en procentstapel, eftersom `restic copy` inte exponerar något maskinläsbart förlopp.

!!! note "Återställ från vilken plats som helst"
    Varje container, VM, filuppsättning, flashen och app-konfigurationen listar sina säkerhetskopior som en enda tidslinje över alla platser en kopia ligger på. En säkerhetskopia kopierad till B2 dyker upp en gång, märkt med varje plats som har den. En återställning tar den första platsen den kan nå, med start i det arkiv objektet skrivs till, och du kan välja en annan plats per rad. Off-site-platser läses bara när du öppnar dem. Att radera på en plats kontrollerar först de andra och säger om det var den sista kopian.

## Placering per objekt {#placement}

Varje kort för container, VM och filuppsättning har en rad **Placering** med tre segment:

- **Lokal** skriver objektet till arkivet som visas under **Sparad på** och kopierar det ingenstans. Använd det för data som redan har en andra kopia, till exempel en resurs som ligger på en NAS.
- **Lokal + utanför platsen** skriver det dit också och kopierar det till målen ikryssade under **Kopiera till**, ett chip per off-site-mål för domänen. Kryssa ur ett chip och det målet får inget nytt från det här objektet.
- **Endast utanför platsen** skriver objektet direkt till lagringsplatsen under **Skicka till**, vilken lagringsplats som helst utom domänens hem. Där domänen redan kopieras till den lagringsplatsen får objektet ett direkt arkiv bredvid kopiorna; annars skapar BombVault ett arkiv för domänen där.

Platsen är fast från objektets första säkerhetskopiering, eftersom BombVault aldrig flyttar säkerhetskopior mellan arkiv. Kopiorna kan ändras när som helst. Ett mål som inte längre får ett objekt behåller de kopior det har och trimmar dem till sin egen retention vid domänens nästa off-site-körning; **Radera hos B2** på kortet tar bort dem direkt. När några av de kopiorna inte finns någon annanstans listar bekräftelsen dem efter datum och ber om objektets namn. Från append-only-mål går det inte att radera.

Under raden berättar kortet vart objektet går och vad som faktiskt finns där: hur många platser som har det, när varje mål senast sågs, och om 3-2-1 är uppfyllt. En plats är servern med originaldata och varje lagringsplats på en annan plats (se [Utanför lokalerna](#off-the-premises-mark)). BombVault kontrollerar kopior och platser; det kontrollerar inte ”två medier”-delen av 3-2-1.

### Standarder per domän

Kortet **Domäner** under Inställningar, Lagring har en rad per domän. **Kopierad till** gäller genast för varje objekt utan eget val, och för projektmapparna i Compose-stackar. När en domän har säkerhetskopior gäller **Sparad i** för ett nytt objekt vid dess första säkerhetskopiering, och att ändra det flyttar inga säkerhetskopior. Innan du sparar namnger raden varje lagringsplats som vinner eller förlorar objekt och hur många ögonblicksbilder det innebär, och frågan har omkopplaren **Tillämpa på objekt utan säkerhetskopior**, som också sätter varje objekt som ännu inte har en säkerhetskopia på den nya standarden. **Undantag** listar objekten med ett eget val.

Att kryssa i en ny lagringsplats under **Kopierad till** gör att den tar emot varje objekt som inte är satt till Lokal. Bekräftelsen säger hur många objekt det är och, där det är känt, hur mycket historik det motsvarar.

### Direkta arkiv

Att välja en lagringsplats under Endast utanför platsen där domänen redan kopieras frågar en gång och skapar sedan ett direkt arkiv bredvid kopiorna, till exempel `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, och pekar objektet mot det. För ett kopieringsmål utan lagringsplats öppnar valet en dialog med en föreslagen adress och ett anslutningstest som inte skapar något, och **Skapa och använd** skapar arkivet. Ett direkt arkiv tar över lagringsplatsens nyckel, lagringsklass, gränser, append-only-inställning och retention, och ändras med dem. När en ny nyckel för lagringsplatsen inte kan öppna det behåller det direkta arkivet den nyckel det har, och sparningen säger det. Dess ögonblicksbilder bär taggen `bv:direct`, och varje annan retention-passering behåller dem, så ett direkt arkiv som förlorat kopplingen till sin lagringsplats åldras aldrig efter de lokala reglerna. En B2-nyckel som är begränsad till en mapp måste täcka lagringsplatsens adress och inte bara domänens mapp, annars når den inte mappen bredvid.

### Utanför lokalerna {#off-the-premises-mark}

En kopia räknas som en egen plats bara när dess lagringsplats står på en annan plats. En lagringsplats i molnet räknas alltid och en mapp på den här Unraid-servern aldrig; för en NAS, en rest-server eller en SFTP-server svarar du på **Var står enheten?** i lagringsplatsens detaljer med **Här i huset** eller **På en annan plats**. Svaret räknas bara för platser och 3-2-1 på korten och Översikten. Det ändrar ingen kopia.

### Efter en ombyggnad

Kopieringsval lever i BombVaults egna inställningar. Efter en ombyggnad via Identifiera utan en återställd `/config` är de borta, och att kopiera allt skulle skicka objekten du hade lämnat ute till B2 igen. Off-site-replikering för varje ombyggd domän pausar därför. Översikten visar det i gult, och domänens rad på kortet Domäner erbjuder **Bekräfta standard** med en förhandsgranskning av vad nästa körning kopierar och namnen i säkerhetskopiorna som saknar en post, vilka du kan lämna ute där. Endast bekräftelsen avslutar pausen; att importera en inställningsfil tar tillbaka regler och standarder men avslutar den inte.

## En domän sparad på en fjärransluten lagringsplats {#remote-primary-repositories}

En domän behöver inte sparas lokalt. Så länge dess säkerhetskopiesökväg inte innehåller några säkerhetskopior kan du välja en fjärransluten lagringsplats under **Sparad i** på kortet Domäner, och domänen säkerhetskopierar då direkt dit, utan lokal kopia och utan kopieringssteg. Fjärrarkivet är då den enda kopian, om inte domänen också kopieras till en annan lagringsplats. Varje fjärransluten lagringsplats har samma skydd:

- **Ett anslutningstest** innan något skrivs.
- **Bandbreddsgränser** för själva säkerhetskopieringen, samma flaggor `--limit-upload` och `--limit-download` som en kopia använder.
- **Append-only-skydd**, verifierat med samma aktiva manipulationstest. Med det påslaget rensar BombVault aldrig arkivet, eftersom inloggningsuppgifterna på den här maskinen inte får kunna radera säkerhetskopians enda kopia.
- **En tillväxtbudget**, hämtad ur samma storlekstrend som Lagringskortet följer.

En domän som sparas på en fjärransluten lagringsplats är källan till sina kopior precis som en lokal; se [Kopior mellan lagringsplatser med olika inloggningsuppgifter](storage-places.md#different-credentials).

!!! note "Inloggningsuppgifterna hör till lagringsplatsen"
    En fjärransluten lagringsplats har sina egna inloggningsuppgifter. En lagringsplats som sattes upp med de gemensamma molnuppgifterna fortsätter att använda dem tills dess åtkomst ändras i dess detaljer.

## Oföränderligt (append-only) off-site

Flagga ett off-site-repo append-only så att ransomware, eller en komprometterad värd, inte kan radera eller skriva om dina säkerhetskopior. Den bortre sidan (en `restic/rest-server` som körs i `--append-only`-läge) **upprätthåller** det. BombVault **verifierar** det bara och visar aldrig grönt enbart på ett konfigurationspåstående.

Fönstret **Lägg till lagringsplats** har ett recept, färdigt att klistra in, för en rest-server i append-only-läge, med en användare för den här BombVault-instansen. På en rest-server-lagringsplats med **Append-only** påslaget kör **Testa append-only** i lagringsplatsens detaljer manipulationstestet mot varje domänsökväg, påslagen kopia och repo på lagringsplatsen och ger ett svar för hela lagringsplatsen, så att append-only off-site är nåbar utan att handredigera konfigurationer.

!!! note "En lyckad radering under `/locks/` är förväntad"
    Append-only betyder inte att ingenting längre kan raderas. restic måste ta och släppa sina egna lås, så `/locks/` förblir avsiktligt skrivbar och raderbar. Ögonblicksbilder och datan bakom dem, alltså precis det som utpressningsprogram skulle gå efter, går inte att ta bort. Testar du motparten själv är en radering som lyckas under `/locks/` korrekt beteende och inte ett hål i skyddet.

!!! warning "Oföränderliga repos rensas aldrig från den här boxen"
    Ett oföränderligt off-site rensar avsiktligt aldrig gamla ögonblicksbilder. Sätt ett **tillväxtbudget-alarm** för det så att du varnas innan repo-storleken skenar iväg.

## Manipulationstest

BombVault bevisar regelbundet append-only-garantin genom att faktiskt försöka en radering mot off-site-repot, riktad mot ett obefintligt objekt:

- **Nekad** betyder skyddad.
- **Accepterad** betyder inte skyddad.
- Ett **obestämt** resultat (server onåbar, autentiseringsfel) vänder aldrig det lagrade utslaget.

En verklig skyddad-till-oskyddad-vändning avfyrar ett enda larm.

På en lagringsplats provar **Testa append-only** varje domänsökväg, påslagen kopia och repo där med dess egna uppgifter och slår ihop utslagen till ett svar: ett enda repo som accepterar en radering gör hela lagringsplatsen till *radering tillåten*.

## DR-övningar

BombVault erbjuder två nivåer av bevis på att dina säkerhetskopior faktiskt går att återställa, inte bara finns.

- **Återställningsverifieringsövningar (lokala).** BombVault kör regelbundet `restic check --read-data-subset` (avgränsad, aldrig en diskfyllande fullständig återställning) och visar en *senast verifierad återställbar*-märkning per domän. Kadensen finns under Inställningar, Scheman; märkningen under Inställningar, Integritet.
- **DR-övningar (off-site).** BombVault återställer ett verkligt mål från off-site-repot till en engångssandlåda, verifierar det fil-för-fil och byte-för-byte, och städar sedan upp. Detta bevisar att du kan återhämta dig från off-site, inte bara att repot svarar. Bara lagringsplatser på en annan plats övas, eftersom en kopia i samma hus inte bevisar något om att förlora huset. En domän som kopieras till flera av dem övas mot en per schemalagd körning, i tur och ordning, och Översikten namnger lagringsplatsen för den senaste övningen.

**Poängkortet för ransomware-skydd** på Översikten rullar upp detta i en grön / gul / röd hållning per domän, med en åldersstämplad checklista (off-site konfigurerat, append-only verifierat, replikering aktuell, återställningsövning godkänd, kryptering på, rensningsstrategi satt). Varje röd rad djuplänkar till åtgärden, och kortet blir grönt endast på verifierade fakta.

## Mottagarpanel (den mottagande sidan)

![Den mottagande sidan, bevakad skrivskyddat, med en integritetskontroll körd på denna maskin.](assets/screenshots/receiver.png)

*Den mottagande sidan, bevakad skrivskyddat, med en integritetskontroll körd på denna maskin.*

Allt ovan är den *sändande* sidan. På boxen som **tar emot** oföränderliga off-site-kopior från en annan BombVault ger mottagarpanelen dig oberoende, skrivskyddad övervakning av de repositorierna på den mottagande hårdvaran, så att ett tyst fel i den bortre änden inte förblir obemärkt.

Slå på **Mottagare**-växeln i Inställningar för att avslöja en **Mottagare**-flik. Den är av som standard; aktivera den endast på en box som faktiskt tar emot oföränderliga off-site-säkerhetskopior. Registrera sedan ett mottaget repository (skrivskyddat, öppnat med den sändande instansens nyckel) för att få:

- **En ögonblicksbildsinventering grupperad per källa**, så att du exakt kan se vilka containrar, VM:ar och filuppsättningar som har landat.
- **Senast mottaget** per källa, så att du vet hur färsk var och en är.
- **En oberoende `restic check`** körd på den mottagande hårdvaran, så att integritet verifieras där datan faktiskt sitter, inte bara på avsändaren.
- **En dödmansknapp:** ett larm när en källa slutar sända inom ett fönster du ställer in.
- **Integritetslarm:** ett larm när en kontroll på den mottagande sidan misslyckas.

Mottagaren är strikt skrivskyddad. Den skriver aldrig till det mottagna repositoriet, så den kan aldrig bryta append-only-garantin som avsändaren förlitar sig på.

## Genomgånget exempel: två Unraid-maskiner, hela vägen

Ovan beskrivs delarna. Här är en komplett uppsättning med riktiga värden, för delar är lättare att sätta ihop när man har sett dem ihopsatta en gång.

Två maskiner: **TOWER** kör containrarna och skickar säkerhetskopiorna, **VAULT** tar emot dem och upprätthåller oföränderligheten. Byt ut mot dina egna namn, adresser och utdelningssökvägar.

**1. Res upp append-only-servern på VAULT.** I BombVault på TOWER, öppna *Inställningar → Lagring*, klicka på **Lägg till lagringsplats**, välj **rest-server** och klicka på **Visa recept**. Kopiera blocket **Unraid-mall**, spara det på VAULT som `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, gå sedan till *Docker → Add Container* och välj **rest-server** i mallistan. Skriv in den visade `htpasswd`-raden i `/mnt/user/appdata/rest-server/.htpasswd` på VAULT innan du startar den. Lösenordet visas en gång och sparas aldrig; receptet har redan fyllt i det och användaren i formuläret på TOWER, så låt det fönstret vara öppet. `htpasswd`-raden bär samma lösenord, redan bcrypt-hashat åt dig, så du behöver inte hasha något själv.

    Låt `--append-only` stå kvar i OPTIONS-fältet. Utan det är VAULT bara en vanlig utdelning igen.

**2. Lägg till lagringsplatsen på TOWER.** Ange VAULTs adress, `http://VAULT:8000`, bredvid användaren och lösenordet som receptet fyllde i, och klicka sedan på **Testa anslutning**. BombVault bygger adressen av dem:

    rest:http://VAULT:8000/tower

Första segmentet i sökvägen är htpasswd-användaren, här `tower`, och varje domän får sin mapp under den, till exempel `rest:http://VAULT:8000/tower/container`. Svara **På en annan plats** på **Var står enheten?**, klicka på **Lägg till** och kryssa i lagringsplatsen under **Kopierad till** för de domäner som ska dit.

**3. Slå på Append-only på TOWER** under **Skydd** i lagringsplatsens detaljer, och klicka sedan på **Testa append-only**. Testet provar varje domänsökväg, kopia och repo på lagringsplatsen och ger ett svar för lagringsplatsen, som måste vara *radering nekad*. Vad svaren betyder:

| Resultat | Vad som hände |
| --- | --- |
| **radering nekad** | VAULT vägrade raderingen. Det är det enda godkända tillståndet. |
| **radering tillåten** | VAULT accepterade en radering. `--append-only` saknas eller har tagits bort. |
| ett meddelande i stället för ett resultat | Testet kunde inte köras. Oftast är adressen inte den restic själv använder, eller så har uppgifterna ändrats. Inget registreras och inget larm utlöses. |

**4. Se på VAULT vad som kommer in.** Slå på *Inställningar → Mottagare*, öppna fliken **Mottagare** och registrera arkivet skrivskyddat.

!!! warning "Platsen är en sökväg **inuti** containern, skriven relativt värdmonteringen"
    Ange `user/appdata/rest-server/tower/container`, **inte** `/mnt/user/appdata/…`. BombVault kör i en container där värdens `/mnt` är monterad någon annanstans; en absolut värdsökväg finns inte där. Klistrar du in en sådan talar BombVault om vilken relativ sökväg du ska använda i stället.

    **Sändande APP_KEY** är TOWERs nyckel, inte VAULTs. Du hittar den på TOWER under *Inställningar → System*.

**5. Gör det ömsesidigt, om du vill.** Upprepa samma fem steg åt andra hållet: en rest-server på TOWER som tar emot VAULTs kopia. Då upprätthåller varje maskin oföränderligheten åt den andra, och ingen kan radera den andras säkerhetskopior.

## Guidad återställning

En dedikerad **Återställning**-flik lotsar en ny eller ombyggd installation genom katastrofscenariot, på ett ställe:

1. **Kontrollerar att BombVault kan läsa dina säkerhetskopior** (krypteringsnyckel-fällan direkt).
2. **Återställer BombVaults egna inställningar**, så att säkerhetskopiesökvägarna, off-site-målen och uppgifterna som resten av flödet behöver kommer förifyllda. Den läser inställningssäkerhetskopian från lagringsplatsen som raden Auto-säkerhetskopia anger under **Sparad i**, eller från Auto-säkerhetskopians kopia under **Kopierad till**, och visar den lagringsplatsen med sin adress; för att läsa från en annan lagringsplats, ändra först raden Auto-säkerhetskopia i steg 3. Återställningen tillämpas via en självomstart över Docker-socketen, så att den körande inställningsdatabasen aldrig skrivs över under ett öppet handtag.
3. **Ansluter dina befintliga säkerhetskopior** via raderna på kortet **Domäner**: på varje domäns rad väljer du lagringsplatsen där dess säkerhetskopior ligger under **Sparad i** och lagringsplatserna med dess kopior under **Kopierad till**. En lagringsplats som ingen rad erbjuder än, till exempel en utdelning, en server eller en molnbucket, ansluts med **Lägg till lagringsplats**, samma fönster som under Inställningar, Lagring. **Anslut och förhandsgranska** kontrollerar sedan att säkerhetskopiorna går att läsa.
4. **Identifierar** containrarna, VM:arna, filuppsättningarna och ZFS-datauppsättningarna lagrade i det.
5. **Återställer containrarna och VM:arna i ett svep** (lämnade stoppade, så att du startar dem medvetet) och listar filuppsättningarna och ZFS-objekten som du återställer ett i taget; ZFS-objekt kommer tillbaka avstängda. Ditt återställningskit är ett klick bort.

!!! note "Off-site-kopior väntar efter en ombyggnad"
    När steg 4 återbygger poster utan de gamla inställningarna pausar off-site-replikeringen för de domänerna tills standardplaceringen bekräftas. Se [Placering per objekt](#placement).

!!! tip "Planerad migrering kontra katastrof"
    Guidad återställning återställer BombVaults egna inställningar från en säkerhetskopia. För en *planerad* flytt till en ny box kan du istället ta med din konfiguration direkt med kortet **Exportera och importera inställningar** (en portabel JSON-fil). Se [Konfiguration](configuration.md#portable-settings-export-and-import).

### Återställ från ett annat BombVault-repo

Ett separat kort på fliken **Återställning** öppnar en *annan* BombVault-instans repo (en resurs monterad under `/mnt`, eller en fjärr-URL) med **den instansens `APP_KEY`**, i en engångs, skrivskyddad session. Bläddra bland containrarna, VM:arna och filuppsättningarna som lagras där, välj en ögonblicksbild och återställ den, och det återställda objektet blir en normal lokal container, VM eller filuppsättning. Inget skrivs någonsin till det andra repot, och dina egna säkerhetskopieringsinställningar förblir orörda (sessionen lever i minnet och löper ut av sig själv). Att flytta en container från server A till server B innebär inte längre att peka om dina repo-inställningar och återställa dem efteråt. Live server-till-server-federation är uttryckligen utanför omfånget; detta är en avsiktlig engångshämtning.

## Återställningskit för krypteringsnyckeln

Detta är delen som gör katastrofåterställning möjlig även när det inte finns någon körande BombVault.

Ett klick laddar ner **huvudnyckeln**, det **härledda restic-lösenordet** och de **exakta repo-platserna och kommandona**, så att du kan återställa direkt med restic-CLI på valfri maskin. En påminnelse på Översikten tjatar tills du har förvarat det.

!!! danger "Förvara återställningskitet bort från servern"
    Kitet innehåller hemligheten som dekrypterar dina säkerhetskopior. Förvara det på en säker plats åtskild från servern (en lösenordshanterare, en utskriven kopia i ett kassaskåp). Om du förlorar både BombVault och `APP_KEY` utan något återställningskit kan dina krypterade säkerhetskopior inte återställas.

!!! warning "Den senaste snapshoten är inte alltid den som ska återställas"
    Sedan restic 0.17 visar `restic snapshots` storleken på varje snapshot. Efter dataförlust kan den senaste snapshoten vara den tömda, så återställ inte en snapshot som är mycket mindre än de före den. Efter ransomware kan det vara den krypterade, i vanlig storlek. Om BombVault fortfarande körs, titta först på sidan **Avvikelser**: den anger den senaste bra säkerhetskopian. En återställning behöver inga avvikelsedata från BombVault, och gallringspausen behåller bara fler snapshots.

### När paketet inte finns till hands

Lösenordet lagras ingenstans, det **beräknas** ur `APP_KEY`. Med nyckeln och ett skal kan du alltså återskapa det själv:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Det är HMAC-SHA256 över den fasta strängen `bombvault:restic-repo`, med de råa byten i den hexadecimala `APP_KEY` som nyckel, utskrivet som 64 gemena hexadecimala tecken. Samma värde står i paketet som det härledda restic-lösenordet; det här är för dagen då paketet ligger någon annanstans än du.

!!! warning "För ett mottaget arkiv, använd den SÄNDANDE instansens nyckel"
    Ett arkiv som kommit hit via off-site-replikering skapades av maskinen som skickade det, med **dess** `APP_KEY`. Att härleda ur den mottagande maskinens nyckel ger ett lösenord som restic avvisar, vilket ser ut precis som ett trasigt arkiv utan att vara det. Det är den vanliga anledningen till att `restic check` på ett mottaget arkiv frågar efter lösenordet gång på gång.

Eftersom återställningsdefinitioner ligger **inuti** varje repo (`<repo>/def`, `<repo>/vm-def`) är en kopierad repo-mapp helt självständig, så kitet plus repot är allt en bare-metal-återställning behöver.

## Hämta tillbaka en databasdump {#database-dumps}

En databasdump är en egen återställningspunkt i containerförrådet, med etiketten `dbdump:<container>` och den enda filen `/dbdump/<container>.sql`. BombVault listar, hämtar och importerar dem under **Säkerhetskopior**; nedan står samma steg med enbart restic, för dagen då BombVault inte finns till hands.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Etiketterna `dbversion:` och `dbname:` på varje dump säger vilken serverversion den kommer från och vilka databaser den rymmer. En komplett fil slutar med `-- PostgreSQL database cluster dump complete` eller `-- Dump completed`.

Importera den i en container med samma eller nyare version (PostgreSQL), eller samma huvudversion (MySQL och MariaDB), startad en gång med tom datamapp så att den initierar sig. Värden behöver ingen databasklient, containern har en:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

För en enda databas ur en full dump tar MySQL och MariaDB `--one-database <name>` på klientkommandot. En PostgreSQL-dump har ett avsnitt per databas, vart och ett inlett med raden `\connect <name>`: kopiera det avsnittet till en egen fil och importera den med `-d <name>` efter att databasen skapats.

!!! warning "En dump tagen som root bär med sig serverns användare"
    En full MySQL- eller MariaDB-dump tagen som root innehåller systemdatabasen `mysql`, så en import ersätter den nya serverns konton, root-lösenordet inräknat, med dem från dumpen. På PostgreSQL är `role ... already exists` för användaren som containern själv skapade väntat och ofarligt.
