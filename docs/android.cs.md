# Aplikace pro Android

Aplikace pro Android vám dá do telefonu všechny servery BombVault z vaší skupiny. Začíná seznamem vašich serverů, nad kterým je protokol aktivit všech z nich, tytéž řádky, jaké ukazuje Přehled, a po klepnutí na server otevře jeho zobrazení pro telefon. Sama aplikace nic nezálohuje.

## Získání aplikace {#install}

- **APK:** každé vydání má na své stránce soubor `bombvault-android.apk` a [tento odkaz](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) vždy stáhne nejnovější sestavení. Aplikace vyžaduje Android 10 nebo novější a Android se jednou zeptá, zda aplikace, kterou soubor otevřete, smí instalovat aplikace.
- **Google Play:** aplikace je v uzavřeném testování, dokud ji nebude možné zveřejnit. Google Play zveřejní aplikaci z nového účtu vývojáře až poté, co ji alespoň 12 testerů mělo 14 dní nainstalovanou. Pokud chcete pomoci, přidejte se do [skupiny testerů](https://groups.google.com/g/arrowloop-testers), otevřete [testovací stránku](https://play.google.com/apps/testing/bombvault.halleluja.design), klepněte na **Stát se testerem** a nainstalujte BombVault z Google Play.
- **F-Droid:** záznam v katalogu přibude později.

Spárujte aplikaci se servery ve verzi 9.7.0 nebo novější. Server se starší verzí může být ve stejné skupině, ale telefon u sebe ukáže jako obyčejnou instanci a aplikace čte aktivitu tohoto serveru až po přihlášení.

## Párování QR kódem {#pairing}

1. Na kterémkoli serveru své skupiny otevřete **Nastavení, Párování** a zvolte **Zobrazit frázi**. Objeví se dvanáct slov a vedle nich QR kód.
2. V aplikaci klepněte na **Naskenovat QR kód** a namiřte telefon na kód. Slova můžete také vložit nebo napsat.
3. Než aplikace cokoli uloží, vypíše servery dané skupiny. **Přidat všech** přidá každý z nich.

Telefon se pak připojí ke skupině jako další instance. Co na jednotlivých serverech běží, čte přes skupinu bez přihlášení, doma přímo a mimo domov přes relay. Jak funguje samotná skupina, popisuje [Párování instancí](offsite-recovery.md#pairing).

!!! note "Rozhraní stále potřebuje cestu k serveru"
    Seznam serverů a protokol aktivit přicházejí přes skupinu. Rozhraní serveru se ale otevírá přímo, takže telefon musí dosáhnout na adresu serveru, doma nebo přes VPN.

## Přihlášení na spárovaném telefonu {#sign-in}

Telefon spárovaný s vaší skupinou otevře každý z jejích serverů rovnou přihlášený. Před načtením stránky si u daného serveru přes skupinu vyžádá relaci a server ji vydá jen členovi, který je telefonem. Server přidaný podle adresy se zeptá na heslo, stejně jako v prohlížeči. Kdo zná dvanáct slov, může už tak otevřít každou zálohu skupiny, takže párování nedává nic nového.

## Servery mimo skupinu {#other-servers}

- **Přidat server** přijímá adresu, kterou BombVault otevíráte v prohlížeči, například `192.168.1.10:3443`. Bez `http://` nebo `https://` na začátku použije aplikace https.
- Servery, které se ohlašují v místní síti, jsou uvedené pod **V této síti** a otevřou se jedním klepnutím. Ohlašují se, dokud je v Nastavení, Integrace zapnuto **Najít v síti**. Server v jiné síti nebo za VPN se tam neobjeví.
- Samopodepsanému certifikátu se důvěřuje jednou podle jeho otisku SHA-256. Když server později ukáže jiný, aplikace vás upozorní a otevře ho až poté, co novému certifikátu důvěřujete.

## Telefon na stránce Instance {#instances}

Telefon dostane na stránce Instance každého serveru ve skupině vlastní kartu, označenou jako aplikace pro Android a pojmenovanou podle názvu, který jste telefonu dali. Nemá vysvědčení, protože nic nezálohuje, a **Odebrat** ho ze stránky odstraní.

## Nastavení {#settings}

Ozubené kolo vedle plusu otevře nastavení aplikace:

- jazyk a název, pod kterým se telefon zobrazuje na stránce Instance (prázdné pole znamená model telefonu),
- vzhled, který se řídí prvním serverem v seznamu, dokud si nenastavíte vlastní, a animace, které mají vlastní nastavení,
- hlášení ke zkopírování, když hlásíte problém; neobsahuje žádnou adresu, název ani frázi,
- kartu O aplikaci se zásadami ochrany osobních údajů,
- **Odebrat všechny servery**, které z aplikace odebere všechny servery a opustí skupinu. Na samotných serverech se nic nezmění.

## Stahování a nahrávání {#files}

Exporty, sady pro obnovu, ZIPy flashe a dumpy databází se ukládají do složky Stažené soubory v telefonu, stejně jako z prohlížeče. Import nastavení otevře výběr souborů v telefonu.

## Na dotykové obrazovce {#touch}

Pod prstem nic nereaguje na najetí, takže ovládací prvek při podržení zeslábne a tlačítko s logem značky se rozsvítí barvou značky, dokud prst nezvednete. Dlouhý stisk tlačítka se počítá jako pomalé klepnutí a neotevře nabídku odkazu.

## Místo toho v prohlížeči {#browser}

Chrome a Edge umí webové rozhraní BombVault nainstalovat jako aplikaci ve vlastním okně, v telefonu stejně jako v počítači. Nic se neukládá do mezipaměti, takže se aktualizace projeví hned.

## Soukromí {#privacy}

Aplikace nemá účty, reklamy ani analytiku a na pozadí nic nespouští. Její [zásady ochrany osobních údajů](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) uvádějí, co ukládá a co kam odesílá.
