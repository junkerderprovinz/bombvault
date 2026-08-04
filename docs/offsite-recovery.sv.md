# Off-site och återställning

Lokala säkerhetskopior skyddar dig mot en förlorad container eller en dålig uppdatering. Off-site-replikering och ett testat återställningskit skyddar dig mot hela boxen, ransomware eller en brand. Den här sidan täcker att replikera off-site, att göra den kopian manipuleringssäker, att bevisa att du kan återställa, och att återhämta dig när BombVault självt är borta.

## Off-site-replikering

Behåll den snabba lokala säkerhetskopian och lägg till en eller flera off-site-repliker. Ange ett repo per domän på fliken **Inställningar, Off-site**. BombVault replikerar nya ögonblicksbilder dit med `restic copy` på best-effort-basis, så att en off-site-hicka aldrig misslyckar den lokala säkerhetskopian. Det lokala repot förblir primärt.

- **Flera off-site-mål per domän.** Varje domän (containrar, VM:ar, flash, config och filuppsättningar) kan replikera till flera off-site-mål samtidigt, inte bara ett, så att du kan hålla, till exempel, en rest-server på en väns box och en S3-bucket parallellt. Lägg till extra mål under Inställningar, Off-site, var och en med sitt eget repository, S3-lagringsklass, append-only-flagga, retention och tillväxtbudget. En befintlig enskild off-site-uppsättning förs över orörd som det första målet, och varje mål i en domän replikeras enligt den domänens off-site-schema.
- **Off-site-schema per domän** (redigerat tillsammans med alla andra scheman under Inställningar, Scheman): lämna det tomt för att replikera efter varje lokal säkerhetskopiering, eller sätt en kadens (till exempel `weekly Sun 03:00`) för att skicka off-site mer sällan än du säkerhetskopierar lokalt. En **Replikera nu**-knapp täcker körningar på begäran.
- **Off-site-retention** finns under Inställningar, Off-site så att du kan behålla off-site-kopior längre som ett arkiv. Lämna policyn helt-noll för att aldrig autotrimma off-site-ögonblicksbilder.
- **Bandbreddsgränser** (Inställningar, Off-site) begränsar restics uppladdnings-/nedladdningshastighet så att replikering inte mättar din WAN.
- En **replikeringsindikator** visar vilken domän som replikeras medan det pågår (på dess sida och Översikten). Det är en aktiv indikator, inte en procentstapel, eftersom `restic copy` inte exponerar något maskinläsbart förlopp.

!!! note "Återställ direkt från off-site"
    Varje säkerhetskopieringsläsare har en omkopplare **Lokal / Off-site**, så om ett lokalt repo förloras eller skadas kan du lista och återställa direkt från off-site-repliken. Radering sker per källa: att ta bort en säkerhetskopia påverkar bara den kopia du tittar på.

## Oföränderligt (append-only) off-site

Flagga ett off-site-repo append-only så att ransomware, eller en komprometterad värd, inte kan radera eller skriva om dina säkerhetskopior. Den bortre sidan (en `restic/rest-server` som körs i `--append-only`-läge) **upprätthåller** det. BombVault **verifierar** det bara och visar aldrig grönt enbart på ett konfigurationspåstående.

Guiden **guidad off-site-uppsättning** lotsar dig från val av backend (rest-server / rclone / S3) genom ett färdigt-att-klistra-in rest-server-deploy-utdrag, ett anslutningstest, den oföränderliga växeln (som kör manipulationstestet omedelbart) och en retention-strategi, så att append-only off-site är nåbar utan att handredigera konfigurationer.

!!! warning "Oföränderliga repos rensas aldrig från den här boxen"
    Ett oföränderligt off-site rensar avsiktligt aldrig gamla ögonblicksbilder. Sätt ett **tillväxtbudget-alarm** för det så att du varnas innan repo-storleken skenar iväg.

## Manipulationstest

BombVault bevisar regelbundet append-only-garantin genom att faktiskt försöka en radering mot off-site-repot, riktad mot ett obefintligt objekt:

- **Nekad** betyder skyddad.
- **Accepterad** betyder inte skyddad.
- Ett **obestämt** resultat (server onåbar, autentiseringsfel) vänder aldrig det lagrade utslaget.

En verklig skyddad-till-oskyddad-vändning avfyrar ett enda larm.

## DR-övningar

BombVault erbjuder två nivåer av bevis på att dina säkerhetskopior faktiskt går att återställa, inte bara finns.

- **Återställningsverifieringsövningar (lokala).** BombVault kör regelbundet `restic check --read-data-subset` (avgränsad, aldrig en diskfyllande fullständig återställning) och visar en *senast verifierad återställbar*-märkning per domän. Kadensen finns under Inställningar, Scheman; märkningen under Inställningar, Integritet.
- **DR-övningar (off-site).** BombVault återställer ett verkligt mål från off-site-repot till en engångssandlåda, verifierar det fil-för-fil och byte-för-byte, och städar sedan upp. Detta bevisar att du kan återhämta dig från off-site, inte bara att repot svarar.

**Poängkortet för ransomware-skydd** på Översikten rullar upp detta i en grön / gul / röd hållning per domän, med en åldersstämplad checklista (off-site konfigurerat, append-only verifierat, replikering aktuell, återställningsövning godkänd, kryptering på, rensningsstrategi satt). Varje röd rad djuplänkar till åtgärden, och kortet blir grönt endast på verifierade fakta.

## Mottagarpanel (den mottagande sidan)

Allt ovan är den *sändande* sidan. På boxen som **tar emot** oföränderliga off-site-kopior från en annan BombVault ger mottagarpanelen dig oberoende, skrivskyddad övervakning av de repositorierna på den mottagande hårdvaran, så att ett tyst fel i den bortre änden inte förblir obemärkt.

Slå på **Mottagare**-växeln i Inställningar för att avslöja en **Mottagare**-flik. Den är av som standard; aktivera den endast på en box som faktiskt tar emot oföränderliga off-site-säkerhetskopior. Registrera sedan ett mottaget repository (skrivskyddat, öppnat med den sändande instansens nyckel) för att få:

- **En ögonblicksbildsinventering grupperad per källa**, så att du exakt kan se vilka containrar, VM:ar och filuppsättningar som har landat.
- **Senast mottaget** per källa, så att du vet hur färsk var och en är.
- **En oberoende `restic check`** körd på den mottagande hårdvaran, så att integritet verifieras där datan faktiskt sitter, inte bara på avsändaren.
- **En dödmansknapp:** ett larm när en källa slutar sända inom ett fönster du ställer in.
- **Integritetslarm:** ett larm när en kontroll på den mottagande sidan misslyckas.

Mottagaren är strikt skrivskyddad. Den skriver aldrig till det mottagna repositoriet, så den kan aldrig bryta append-only-garantin som avsändaren förlitar sig på.

## Guidad återställning

En dedikerad **Återställning**-flik lotsar en ny eller ombyggd installation genom katastrofscenariot, på ett ställe:

1. **Återställer BombVaults egna inställningar först**, så att säkerhetskopiesökvägarna, off-site-målen och uppgifterna som resten av flödet behöver kommer förifyllda (tillämpade via en självomstart över Docker-socketen, så att den körande inställningsdatabasen aldrig skrivs över under ett öppet handtag).
2. **Kontrollerar att BombVault kan läsa dina säkerhetskopior** (krypteringsnyckel-fällan direkt).
3. Låter dig **peka mot ditt befintliga repo** (lokalt eller off-site).
4. **Identifierar** containrarna, VM:arna och filuppsättningarna lagrade i det.
5. **Återställer dem alla** (lämnade stoppade, så att du startar dem medvetet), med ditt återställningskit ett klick bort.

!!! tip "Planerad migrering kontra katastrof"
    Guidad återställning återställer BombVaults egna inställningar från en säkerhetskopia. För en *planerad* flytt till en ny box kan du istället ta med din konfiguration direkt med kortet **Exportera och importera inställningar** (en portabel JSON-fil). Se [Konfiguration](configuration.md#portable-settings-export-and-import).

### Återställ från ett annat BombVault-repo

Ett separat kort på fliken **Återställning** öppnar en *annan* BombVault-instans repo (en resurs monterad under `/mnt`, eller en fjärr-URL) med **den instansens `APP_KEY`**, i en engångs, skrivskyddad session. Bläddra bland containrarna, VM:arna och filuppsättningarna som lagras där, välj en ögonblicksbild och återställ den, och det återställda objektet blir en normal lokal container, VM eller filuppsättning. Inget skrivs någonsin till det andra repot, och dina egna säkerhetskopieringsinställningar förblir orörda (sessionen lever i minnet och löper ut av sig själv). Att flytta en container från server A till server B innebär inte längre att peka om dina repo-inställningar och återställa dem efteråt. Live server-till-server-federation är uttryckligen utanför omfånget; detta är en avsiktlig engångshämtning.

## Återställningskit för krypteringsnyckeln

Detta är delen som gör katastrofåterställning möjlig även när det inte finns någon körande BombVault.

Ett klick laddar ner **huvudnyckeln**, det **härledda restic-lösenordet** och de **exakta repo-platserna och kommandona**, så att du kan återställa direkt med restic-CLI på valfri maskin. En påminnelse på Översikten tjatar tills du har förvarat det.

!!! danger "Förvara återställningskitet bort från servern"
    Kitet innehåller hemligheten som dekrypterar dina säkerhetskopior. Förvara det på en säker plats åtskild från servern (en lösenordshanterare, en utskriven kopia i ett kassaskåp). Om du förlorar både BombVault och `APP_KEY` utan något återställningskit kan dina krypterade säkerhetskopior inte återställas.

Eftersom återställningsdefinitioner ligger **inuti** varje repo (`<repo>/def`, `<repo>/vm-def`) är en kopierad repo-mapp helt självständig, så kitet plus repot är allt en bare-metal-återställning behöver.
