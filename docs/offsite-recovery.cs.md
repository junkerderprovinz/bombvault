# Mimo lokalitu a obnova

Místní zálohy vás chrání před ztraceným kontejnerem nebo špatnou aktualizací. Replikace mimo lokalitu a otestovaná sada pro obnovu vás chrání před celým strojem, ransomwarem nebo požárem. Tato stránka pokrývá replikaci mimo lokalitu, zajištění odolnosti té kopie proti manipulaci, prokázání, že umíte obnovit, a obnovu, když je samotný BombVault pryč.

## Replikace mimo lokalitu

Ponechte rychlou místní zálohu a přidejte jednu nebo více replik mimo lokalitu. Nastavte repozitář na doménu v záložce **Nastavení, Mimo lokalitu**. BombVault tam replikuje nové snímky pomocí `restic copy` na základě nejlepší snahy, takže zádrhel mimo lokalitu nikdy nezhatí místní zálohu. Místní repozitář zůstává primární.

- **Více cílů mimo lokalitu na doménu.** Každá doména (kontejnery, VM, flash, config a sady souborů) může replikovat na několik cílů mimo lokalitu najednou, ne jen na jeden, takže můžete držet například rest-server na stroji kamaráda a S3 bucket paralelně. Přidejte další cíle v Nastavení, Mimo lokalitu, každý s vlastním repozitářem, třídou úložiště S3, příznakem append-only, uchováváním a rozpočtem růstu. Stávající jednotlivé nastavení mimo lokalitu se nedotčeno přenese jako první cíl a každý cíl domény replikuje podle plánu mimo lokalitu dané domény.
- **Plán mimo lokalitu na doménu** (upravovaný spolu s každým dalším plánem v Nastavení, Plány): ponechte prázdný pro replikaci po každé místní záloze, nebo nastavte kadenci (například `weekly Sun 03:00`) pro odesílání mimo lokalitu méně často, než zálohujete místně. Tlačítko **Replikovat nyní** pokrývá běhy na vyžádání.
- **Uchovávání mimo lokalitu** žije v Nastavení, Mimo lokalitu, takže můžete kopie mimo lokalitu držet déle jako archiv. Ponechte zásadu celou na nule, aby se snímky mimo lokalitu nikdy automaticky neprořezávaly.
- **Limity šířky pásma** (Nastavení, Mimo lokalitu) omezují rychlost nahrávání/stahování restic, aby replikace nezasytila vaše WAN.
- **Indikátor replikace** zobrazuje, která doména právě replikuje, zatímco běží (na její stránce a na Přehledu). Je to aktivní indikátor, nikoli procentuální panel, protože `restic copy` nezpřístupňuje žádný strojově čitelný průběh.

!!! note "Obnova přímo z mimo lokalitu"
    Každý prohlížeč záloh má přepínač **Místní / Mimo lokalitu**, takže pokud se místní repozitář ztratí nebo poškodí, můžete vypsat a obnovit přímo z repliky mimo lokalitu. Mazání je pro každý zdroj zvlášť: odstranění zálohy ovlivní jen kopii, kterou si prohlížíte.

## Vzdálené primární repozitáře {#remote-primary-repositories}

Cesta zálohy domény (Nastavení, Cesty a úložiště) se neomezuje na místní složku: nasměrujte ji rovnou na vzdálený repozitář resticu (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:uživatel@host:/repo`, `rclone:remote:bucket/cesta`) a BombVault zálohuje přímo tam, bez samostatné místní kopie a bez kroku replikace. Je to opravdu jiný tvar než replikace mimo lokalitu výše: tam je primární místní repozitář a ten mimo lokalitu je jeho archivem podle možností; zde **je** primární ten vzdálený a je jedinou kopií, dokud pro tuto doménu nenastavíte i replikaci mimo lokalitu (nebo druhý vzdálený repozitář).

Každé z pěti polí cesty (Kontejnery, Virtuální stroje, Flash, Konfigurace, Soubory) má hned vedle přepínač **Místní / Vzdálené**:

- **Místní** zobrazí známý prohlížeč složek.
- **Vzdálené** jej vymění za prosté pole URL a tlačítko, které otevře stejné okno testu připojení a přihlašovacích údajů, jaké používají cíle mimo lokalitu, jen nastavené pro tento primární repozitář. Odtud získáte:
    - **Test připojení** proti skutečné cestě, dřív než se na ni spolehnete.
    - **Omezení šířky pásma** (odesílání a stahování), aby plánovaná záloha do vzdáleného primárního repozitáře nezahltila vaši linku WAN: tytéž přepínače resticu `--limit-upload` a `--limit-download`, které používá replikace mimo lokalitu, uplatněné na zálohu samotnou.
    - **Ochranu append-only (neměnnost)**, ověřenou stejným aktivním testem manipulace (skutečná sonda DELETE proti druhé straně), jaký dostávají cíle mimo lokalitu. Když je zapnutá, BombVault odmítne repozitář prořezávat: protože za ním není samostatná místní kopie, přihlašovací údaje na tomto stroji nesmějí být schopné smazat jedinou kopii zálohy.
    - **Výstrahu rozpočtu růstu**, odvozenou ze stejného trendu velikosti repozitáře, který karta Úložiště už sleduje.

Nic z toho není povinné: ručně zadaná vzdálená cesta bez uložených bezpečnostních nastavení zálohuje přesně jako dosud (neomezená šířka pásma, lze prořezávat, žádná výstraha rozpočtu). Bezpečnostní okno je tu pro chvíli, kdy chcete stejnou ochranu, jakou dostává kopie mimo lokalitu, aniž byste kvůli tomu museli zakládat samostatný cíl mimo lokalitu.

!!! note "Přihlašovací údaje ke cloudu a REST jsou sdílené"
    Vzdálený primární repozitář se ověřuje stejnými údaji S3/REST, které jsou nastavené v Nastavení, Mimo lokalitu, Přihlašovací údaje ke cloudu. Samostatné úložiště údajů pro primární repozitáře neexistuje.

## Neměnné (append-only) mimo lokalitu

Označte repozitář mimo lokalitu jako append-only, aby ransomware nebo kompromitovaný hostitel nemohl smazat nebo přepsat vaše zálohy. Druhá strana (`restic/rest-server` běžící v režimu `--append-only`) to **vynucuje**. BombVault to pouze **ověřuje** a nikdy nezobrazí zelenou jen na základě konfiguračního tvrzení.

Průvodce **řízeného nastavení mimo lokalitu** vás provede od volby backendu (rest-server / rclone / S3) přes připravený úryvek pro nasazení rest-serveru, test připojení, přepínač neměnnosti (který spustí test odolnosti okamžitě) a strategii uchovávání, takže append-only mimo lokalitu je dosažitelné bez ručního editování konfigurací.

!!! warning "Neměnné repozitáře se z tohoto stroje nikdy neprořezávají"
    Neměnné mimo lokalitu záměrně nikdy neprořezává staré snímky. Nastavte pro něj **alarm rozpočtu růstu**, abyste byli upozorněni dříve, než se velikost repozitáře vymkne kontrole.

## Test odolnosti proti manipulaci

BombVault pravidelně dokazuje záruku append-only tím, že skutečně zkusí mazání proti repozitáři mimo lokalitu, zaměřené na neexistující objekt:

- **Odmítnuto** znamená chráněno.
- **Přijato** znamená nechráněno.
- **Neprůkazný** výsledek (server nedosažitelný, chyba autentizace) nikdy nezmění uložený verdikt.

Skutečný přechod z chráněno na nechráněno spustí jediné upozornění.

## Cvičné obnovy po havárii

BombVault nabízí dvě úrovně důkazu, že vaše zálohy jsou skutečně obnovitelné, nejen přítomné.

- **Cvičné obnovy s ověřením (místní).** BombVault pravidelně spouští `restic check --read-data-subset` (omezené, nikdy plná obnova zaplňující disk) a zobrazuje odznak *naposledy ověřeno jako obnovitelné* na doménu. Kadence žije v Nastavení, Plány; odznak v Nastavení, Integrita.
- **Cvičné obnovy po havárii (mimo lokalitu).** BombVault obnoví skutečný cíl z repozitáře mimo lokalitu do jednorázového sandboxu, ověří jej soubor po souboru a bajt po bajtu, poté ukliďte. To dokazuje, že umíte obnovit z mimo lokalitu, nejen že repozitář odpovídá.

**Vysvědčení ochrany proti ransomwaru** na Přehledu to shrne do zeleného / oranžového / červeného postoje na doménu, s kontrolním seznamem s věkovou značkou (mimo lokalitu nakonfigurováno, append-only ověřeno, replikace aktuální, cvičná obnova prošla, šifrování zapnuto, strategie prořezávání nastavena). Každý červený řádek odkazuje přímo na opravu a karta se rozsvítí zeleně jen na ověřených faktech.

## Řídicí panel příjemce (přijímací strana)

Vše výše je *odesílající* strana. Na stroji, který **přijímá** neměnné kopie mimo lokalitu z jiného BombVaultu, vám řídicí panel příjemce dává nezávislé monitorování těchto repozitářů jen pro čtení na přijímacím hardwaru, takže tiché selhání na druhém konci nezůstane bez povšimnutí.

Zapněte přepínač **Příjemce** v Nastavení k odhalení záložky **Příjemce**. Ve výchozím stavu je vypnuto; zapněte jej jen na stroji, který skutečně přijímá neměnné zálohy mimo lokalitu. Poté zaregistrujte přijatý repozitář (jen pro čtení, otevřený klíčem odesílající instance) pro získání:

- **Inventáře snímků seskupeného podle zdroje**, takže vidíte přesně, které kontejnery, VM a sady souborů dorazily.
- **Naposledy přijato** na zdroj, takže víte, jak čerstvý každý je.
- **Nezávislého `restic check`** spuštěného na přijímacím hardwaru, takže integrita je ověřena tam, kde data skutečně leží, nejen na odesílateli.
- **Pojistky mrtvého muže:** upozornění, když zdroj přestane odesílat v okně, které nastavíte.
- **Upozornění na integritu:** upozornění, když kontrola na přijímací straně selže.

Příjemce je striktně jen pro čtení. Nikdy nezapisuje do přijatého repozitáře, takže nikdy nemůže porušit záruku append-only, na kterou se odesílatel spoléhá.

## Kompletní příklad: dva stroje Unraid, od začátku do konce

Výše jsou popsány jednotlivé díly. Tohle je jedno úplné nastavení se skutečnými hodnotami, protože díly se skládají snáz, když je člověk jednou viděl složené.

Dva stroje: **TOWER** provozuje kontejnery a posílá zálohy, **VAULT** je přijímá a vynucuje neměnnost. Dosaďte vlastní názvy, adresy a cesty ke sdílení.

**1. Na VAULT postavte server v režimu append-only.** V BombVaultu na TOWER jděte do *Nastavení → Mimo pracoviště → průvodce*, zvolte **rest-server** a vygenerujte recept. Zkopírujte kartu **Šablona Unraid (XML)**, uložte ji na VAULT jako `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, pak *Docker → Add Container* a vyberte **rest-server** ze seznamu šablon. Před spuštěním zapište zobrazený řádek `htpasswd` na VAULT do `/mnt/user/appdata/rest-server/.htpasswd`. Jednorázové heslo se zobrazí jen jednou a nikdy se neukládá: zkopírujte si ho teď.

    Nechte `--append-only` v poli OPTIONS. O to tu celou dobu jde: bez toho je VAULT zase obyčejné sdílení.

**2. Na TOWER na něj nasměrujte vzdálený repozitář.** URL repozitáře má tvar, který recept vypíše:

    rest:http://VAULT:8000/bombvault-containers/containers

První část cesty je uživatel htpasswd, druhá je repozitář. Zadejte vygenerovaného uživatele a heslo jako přihlašovací údaje REST pro cíl a spusťte **test připojení**.

**3. Na TOWER zapněte «Neměnné».** Test manipulace proběhne hned a musí hlásit *chráněno*. Co odpovědi znamenají:

| Výsledek | Co se stalo |
| --- | --- |
| **chráněno** | VAULT smazání odmítl. To je jediný vyhovující stav. |
| **NENÍ chráněno** | VAULT smazání přijal. Chybí `--append-only`, nebo byl odebrán. |
| **neprůkazné** | Ani jedno. Obvykle URL není ta, kterou používá sám restic, nebo se změnily přihlašovací údaje. Nic se nezaznamená a nespustí se žádné upozornění. |

**4. Na VAULT sledujte, co přichází.** Zapněte *Nastavení → Příjemce*, otevřete kartu **Příjemce** a zaregistrujte repozitář jen pro čtení.

!!! warning "Umístění je cesta **uvnitř** kontejneru, zapsaná relativně k připojení hostitele"
    Zadejte `user/appdata/rest-server/bombvault-containers/containers`, **ne** `/mnt/user/appdata/…`. BombVault běží v kontejneru, kde je `/mnt` hostitele připojeno jinde; absolutní cesta hostitele tam neexistuje. Když ji vložíte, BombVault vám nyní sdělí relativní cestu, kterou máte použít.

    **Odesílající APP_KEY** je klíč stroje TOWER, ne VAULT. Najdete jej na TOWER v *Nastavení → Systém*.

**5. Pokud chcete, udělejte to oboustranně.** Zopakujte stejných pět kroků opačným směrem: rest-server na TOWER přijímající kopii z VAULT. Každý stroj pak vynucuje neměnnost pro ten druhý a ani jeden nemůže smazat zálohy toho druhého.

## Řízená obnova

Vyhrazená záložka **Obnova** provede čistou nebo znovu sestavenou instalaci havarijním případem, na jednom místě:

1. **Nejprve obnoví vlastní nastavení BombVaultu**, takže zálohovací cesty, cíle mimo lokalitu a přihlašovací údaje, které zbytek postupu potřebuje, přijdou předvyplněné (aplikováno přes sebe-restart přes Docker socket, takže se živá databáze nastavení nikdy nepřepisuje pod otevřeným handlem).
2. **Zkontroluje, že BombVault umí číst vaše zálohy** (zádrhel se šifrovacím klíčem hned zkraje).
3. Nechá vás **nasměrovat na váš existující repozitář** (místní nebo mimo lokalitu).
4. **Objeví** kontejnery, VM a sady souborů v něm uložené.
5. **Obnoví je všechny** (ponechané zastavené, takže je spustíte záměrně), s vaší sadou pro obnovu na jedno kliknutí.

!!! tip "Plánovaná migrace versus havárie"
    Řízená obnova obnovuje vlastní nastavení BombVaultu ze zálohy. Pro *plánovaný* přesun na nový stroj můžete místo toho přenést konfiguraci přímo pomocí karty **Export a import nastavení** (přenosný soubor JSON). Viz [Konfigurace](configuration.md#portable-settings-export-and-import).

### Obnova z jiného BombVault repozitáře

Samostatná karta v záložce **Obnova** otevře repozitář *jiné* instance BombVaultu (sdílená složka připojená pod `/mnt`, nebo vzdálená URL) s **`APP_KEY` dané instance**, v jednorázové relaci jen pro čtení. Procházejte kontejnery, VM a sady souborů tam uložené, vyberte snímek a obnovte jej, a obnovený objekt se stane běžným místním kontejnerem, VM nebo sadou souborů. Do druhého repozitáře se nikdy nic nezapíše a vaše vlastní nastavení záloh zůstane nedotčeno (relace žije v paměti a sama vyprší). Přesun kontejneru ze serveru A na server B už neznamená přesměrovávat nastavení repozitáře a poté je vracet zpět. Živá federace server-server je explicitně mimo rozsah; toto je záměrné jednorázové stažení.

## Sada pro obnovu šifrovacího klíče

Toto je díl, který umožňuje zotavení po havárii, i když neběží žádný BombVault.

Jedno kliknutí stáhne **hlavní klíč**, **odvozené heslo restic** a **přesná umístění a příkazy repozitáře**, takže můžete obnovit přímo pomocí restic CLI na libovolném stroji. Připomínka na Přehledu vás popohání, dokud si ji neuložíte.

!!! danger "Uložte sadu pro obnovu mimo server"
    Sada obsahuje tajemství, které dešifruje vaše zálohy. Uchovejte ji na bezpečném místě odděleně od serveru (správce hesel, tištěná kopie v trezoru). Pokud ztratíte jak BombVault, tak `APP_KEY` bez sady pro obnovu, vaše šifrované zálohy nelze obnovit.

### Když sada není po ruce

Heslo není nikde uloženo, **počítá se** z `APP_KEY`. S klíčem a shellem si je tedy dokážete odvodit sami:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Je to HMAC-SHA256 nad pevným řetězcem `bombvault:restic-repo`, klíčem jsou syrové bajty šestnáctkového `APP_KEY`, výstup je 64 malých šestnáctkových znaků. Stejná hodnota je v sadě jako odvozené heslo restic; tohle je pro den, kdy sada leží jinde než vy.

!!! warning "U přijatého úložiště použijte klíč ODESÍLAJÍCÍ instance"
    Úložiště, které sem přišlo replikací mimo lokalitu, vytvořil stroj, který je odeslal, svým **vlastním** `APP_KEY`. Odvození z klíče přijímajícího stroje dá heslo, které restic odmítne, což vypadá přesně jako poškozené úložiště, aniž by jím bylo. To je obvyklý důvod, proč se `restic check` na přijatém úložišti stále dokola ptá na heslo.

Protože definice pro obnovu žijí **uvnitř** každého repozitáře (`<repo>/def`, `<repo>/vm-def`), je zkopírovaná složka repozitáře plně soběstačná, takže sada plus repozitář jsou vším, co obnova na holém železe potřebuje.
