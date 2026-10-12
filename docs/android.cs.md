# Aplikace pro Android

Aplikace pro Android ti dá do telefonu všechny servery BombVault z tvé skupiny. Začíná seznamem tvých serverů, nad kterým je protokol aktivit všech z nich, tytéž řádky, jaké ukazuje Přehled, a po klepnutí na server otevře jeho zobrazení pro telefon. Sama aplikace nic nezálohuje.

## Získání aplikace {#install}

- **Nastavení, Aplikace:** karta aplikace pro Android nabízí APK pro vydání, které tvůj server provozuje, s QR kódem k naskenování z telefonu.
- **APK:** každé vydání má na své stránce soubor `bombvault-android.apk` a [tento odkaz](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) vždy stáhne nejnovější sestavení. Aplikace vyžaduje Android 10 nebo novější a Android se jednou zeptá, zda aplikace, kterou soubor otevři, smí instalovat aplikace.
- **Google Play:** aplikace je v uzavřeném testování, dokud ji nebude možné zveřejnit. Google Play zveřejní aplikaci z nového účtu vývojáře až poté, co ji alespoň 12 testerů mělo 14 dní nainstalovanou. Pokud chceš pomoci, přidej se do [skupiny testerů](https://groups.google.com/g/arrowloop-testers), otevři [testovací stránku](https://play.google.com/apps/testing/bombvault.halleluja.design), klepni na **Stát se testerem** a nainstaluj BombVault z Google Play.
- **F-Droid:** záznam v katalogu přibude později.

Spáruj aplikaci se servery ve verzi 9.7.0 nebo novější. Server se starší verzí může být ve stejné skupině, ale telefon u sebe ukáže jako obyčejnou instanci a aplikace čte aktivitu tohoto serveru až po přihlášení.

## Párování QR kódem {#pairing}

1. Na kterémkoli serveru své skupiny otevři **Nastavení, Párování** a zvol **Zobrazit frázi**. Objeví se dvanáct slov a vedle nich QR kód.
2. V aplikaci klepni na **Naskenovat QR kód** a namiř telefon na kód. Slova můžeš také vložit nebo napsat.
3. Než aplikace cokoli uloží, vypíše servery dané skupiny. **Přidat všech** přidá každý z nich.

Telefon se pak připojí ke skupině jako další instance. Co na jednotlivých serverech běží, čte přes skupinu bez přihlášení, doma přímo a mimo domov přes relay. Jak funguje samotná skupina, popisuje [Párování instancí](offsite-recovery.md#pairing).

Aplikace se ke skupině připojuje přes relay, i doma. Pokud je tvoje instance nastavená na **Bez přeposílače**, vyber v **Nastavení, Párování** relay dřív, než kód naskenuješ.

## Mimo domov {#away}

Aplikace zkusí server nejdřív na jeho vlastní adrese. Když neodpoví, třeba v mobilní síti, otevře rozhraní serveru místo toho přes relay skupiny a pod stránkou se objeví řádek **Otevřeno přes přeposílač**. Stejně se otevře server skupiny, jehož adresu aplikace ještě nezná. Pokud neodpoví ani adresa, ani relay, uvede to úvodní obrazovka na kartě serveru.

Přes relay stránka dosáhne na to, na co spárovaný telefon dosáhne doma, jak je popsáno níže. Přihlášení heslem a ukládání souborů fungují jen na vlastní adrese serveru. Ukazatele průběhu se obnovují každé dvě sekundy, ne průběžně. Soubory rozhraní zůstanou po první návštěvě v telefonu, takže se další návštěvy načtou rychleji. Server starší než 10.0.0 se otevře jen na vlastní adrese.

Aplikace přijme odpověď jen tehdy, když ji server podepsal klíčem, který si aplikace připnula při prvním setkání s tímto serverem. Odpoví-li server později jiným klíčem, úvodní obrazovka ukáže otisk nového klíče a stránka se otevře až poté, co mu důvěřuješ.

## Přihlášení na spárovaném telefonu {#sign-in}

Telefon spárovaný s tvou skupinou otevře každý z jejích serverů rovnou přihlášený. Před načtením stránky si u daného serveru přes skupinu vyžádá relaci a server ji vydá jen členovi, který je telefonem. Server přidaný podle adresy se zeptá na heslo, stejně jako v prohlížeči. 

Spárovaný telefon nedostane všechno, co dává heslo. Jeho relace sahá k pevnému seznamu toho, co aplikace ukazuje: přehled, běhy a protokol aktivit, zálohy k prohlížení, spuštění a obnově, anomálie ke čtení a stránku Instance k prohlížení. Cíle a příkazy se vrací bez adres a tajemství. Obnova, která nahrazuje originál, nejdřív zazálohuje aktuální stav. Klíče, sada pro obnovu, úschova klíčů, nastavení a mazání záloh zůstávají u přihlášení přímo na serveru a všechno ostatní odpoví, že toto přihlášení potřebuje. Přes relay platí stejný seznam, i na serveru bez hesla.

Přepínač je **Telefony se mohou spárovat s dvanácti slovy skupiny** v kartě **Aplikace pro Android** v části **Nastavení, Zabezpečení**. Reset hesla ho vypne a odhlásí všechny telefony, dokud ho tam zase nezapneš.

## Servery mimo skupinu {#other-servers}

- **Přidat server** přijímá adresu, kterou BombVault otevíráš v prohlížeči, například `192.168.1.10:3443`. Bez `http://` nebo `https://` na začátku použije aplikace https.
- Servery, které se ohlašují v místní síti, jsou uvedené pod **V této síti** a otevřou se jedním klepnutím. Ohlašují se, dokud je zapnuto **Najít v síti** v kartě **Služby** v části **Nastavení, Integrace**. Server v jiné síti nebo za VPN se tam neobjeví.
- Samopodepsanému certifikátu se důvěřuje jednou podle jeho otisku SHA-256. Když server později ukáže jiný, aplikace tě upozorní a otevře ho až poté, co novému certifikátu důvěřuješ.

## Telefon na stránce Instance {#instances}

Telefon dostane na stránce Instance každého serveru ve skupině vlastní kartu, označenou jako aplikace pro Android a pojmenovanou podle názvu, který jsi telefonu dal. Nemá vysvědčení, protože nic nezálohuje, a **Odebrat** ho ze stránky odstraní.

## Nastavení {#settings}

Ozubené kolo vedle plusu otevře nastavení aplikace:

- jazyk a název, pod kterým se telefon zobrazuje na stránce Instance (prázdné pole znamená model telefonu),
- vzhled, který se řídí prvním serverem v seznamu, dokud si nenastavíš vlastní, a animace, které mají vlastní nastavení,
- hlášení ke zkopírování, když hlásíš problém; neobsahuje žádnou adresu, název ani frázi,
- kartu O aplikaci se zásadami ochrany osobních údajů,
- **Odebrat všechny servery**, které z aplikace odebere všechny servery a opustí skupinu. Na samotných serverech se nic nezmění.

## Stahování a nahrávání {#files}

Exporty, sady pro obnovu, ZIPy flashe a dumpy databází se ukládají do složky Stažené soubory v telefonu, stejně jako z prohlížeče. Import nastavení otevře výběr souborů v telefonu. Přes relay se nic neukládá; otevři k tomu server na jeho vlastní adrese.

## Na dotykové obrazovce {#touch}

Pod prstem nic nereaguje na najetí, takže ovládací prvek při podržení zeslábne a tlačítko s logem značky se rozsvítí barvou značky, dokud prst nezvedneš. Dlouhý stisk tlačítka se počítá jako pomalé klepnutí a neotevře nabídku odkazu.

## Místo toho v prohlížeči {#browser}

Chrome a Edge umí webové rozhraní BombVault nainstalovat jako aplikaci ve vlastním okně, v telefonu stejně jako v počítači. Nic se neukládá do mezipaměti, takže se aktualizace projeví hned.

## Soukromí {#privacy}

Aplikace nemá účty, reklamy ani analytiku a na pozadí nic nespouští. Její [zásady ochrany osobních údajů](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) uvádějí, co ukládá a co kam odesílá.
