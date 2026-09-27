# Mimo lokalitu a obnova

Místní zálohy vás chrání před ztraceným kontejnerem nebo špatnou aktualizací. Replikace mimo lokalitu a otestovaná sada pro obnovu vás chrání před celým strojem, ransomwarem nebo požárem. Tato stránka pokrývá replikaci mimo lokalitu, zajištění odolnosti té kopie proti manipulaci, prokázání, že umíte obnovit, a obnovu, když je samotný BombVault pryč.

## Replikace mimo lokalitu

Ponechte rychlou místní zálohu a kopírujte ji na jedno nebo více dalších míst. Místa, na která se doména kopíruje, zvolíte na kartě **Domény** v **Nastavení, Úložiště**, jeden čip na místo (viz [Úložná místa](storage-places.md#domains)). BombVault tam kopíruje nové snímky pomocí `restic copy` na základě nejlepší snahy, takže nepovedená kopie nikdy nezhatí místní zálohu. Místo, na kterém je doména uložena, nemusí být místní; viz [Doména uložená na vzdáleném místě](#remote-primary-repositories).

- **Několik míst pro kopie na doménu.** Doménu lze kopírovat na několik míst najednou, například na rest-server u kamaráda doma a do bucketu B2. Uchovávání, třída úložiště, append-only, limity a rozpočet růstu patří místu, takže každá kopie se řídí pravidly místa, kam dorazí.
- **Plán kopírování na doménu** (upravovaný spolu s každým dalším plánem v Nastavení, Plány): ponechte prázdný pro kopírování po každé místní záloze, nebo nastavte kadenci (například `weekly Sun 03:00`) pro kopírování méně často, než zálohujete. **Kopírovat nyní** na řádku domény ho spustí na vyžádání.
- **Uchovávání pro každé místo.** Každé místo má vlastní pravidla, takže místo mimo lokalitu může uchovávat kopie déle jako archiv. Místo se všemi pravidly na nule nikdy nic neprořezává.
- **Limity šířky pásma** pro každé místo omezují rychlost nahrávání a stahování restic, aby kopírování nezasytilo vaše WAN.
- **Indikátor replikace** zobrazuje, která doména právě kopíruje, zatímco běží (na její stránce a na Přehledu). Je to aktivní indikátor, nikoli procentuální panel, protože `restic copy` nezpřístupňuje žádný strojově čitelný průběh.

!!! note "Obnova z libovolného místa"
    Každý kontejner, VM, sada složek, flash i konfigurace aplikace vypisují své zálohy jako jednu časovou osu napříč všemi místy, kde záloha leží. Záloha zkopírovaná do B2 se objeví jednou, označená každým místem, které ji drží. Obnova vezme první dosažitelné místo, počínaje repozitářem, do kterého se položka zapisuje, a u každého řádku můžete zvolit jiné místo. Místa mimo lokalitu se čtou, jen když je otevřete. Mazání na jednom místě nejdřív zkontroluje ostatní a řekne, jestli to byla poslední kopie.

## Umístění pro jednotlivé položky {#placement}

Každá karta kontejneru, VM a sady složek má řádek **Umístění** se třemi segmenty:

- **Místní** zapíše položku do repozitáře zobrazeného pod **Uloženo na** a nikam ji nekopíruje. Použijte to pro data, která už mají druhou kopii, například sdílenou složku, jež žije na NAS.
- **Místní + mimo lokalitu** ji zapíše i tam a zkopíruje ji do cílů zaškrtnutých pod **Kopírovat do**, jeden čip na každý cíl domény mimo lokalitu. Odškrtněte čip a ten cíl už od této položky nedostane nic nového.
- **Jen mimo lokalitu** zapíše položku přímo na místo pod **Odeslat do**, tedy na kterékoli místo kromě místa uložení domény. Pokud se doména na toto místo už kopíruje, dostane položka přímý repozitář vedle kopií; jinak tam BombVault pro doménu vytvoří repozitář.

Umístění je pevné od první zálohy položky, protože BombVault nikdy nepřesouvá zálohy mezi repozitáři. Kopie se mohou kdykoli změnit. Cíl, který už položku nedostává, si ponechá kopie, které má, a při dalším běhu mimo lokalitu dané domény je zkrátí podle vlastního uchovávání; **Smazat v B2** na kartě je odstraní okamžitě. Pokud některé z těchto kopií neexistují nikde jinde, potvrzení je vypíše podle data a požádá o název položky. Z cílů append-only mazat nelze.

Pod řádkem karta ukazuje, kam položka směřuje a co tam skutečně je: na kolika lokalitách leží, kdy byl každý cíl naposledy viděn a jestli je splněno 3-2-1. Lokalita je server s původními daty a každé místo v jiné lokalitě (viz [Mimo objekt](#off-the-premises-mark)). BombVault kontroluje kopie a lokality; část 3-2-1 o "dvou médiích" nekontroluje.

### Výchozí nastavení pro doménu

Karta **Domény** v Nastavení, Úložiště má jeden řádek na doménu. **Kopírováno do** platí okamžitě pro každou položku bez vlastní volby a pro projektové složky Compose stacků. Jakmile má doména zálohy, platí **Uloženo v** pro novou položku při její první záloze a jeho změna žádné zálohy nepřesune. Před uložením řádek jmenuje každé místo, které získává nebo ztrácí položky, a kolik snímků to znamená, a otázka nese přepínač **Použít na položky bez záloh**, který na nové výchozí nastavení převede i každou položku, jež zatím nemá zálohu. **Výjimky** vypisují položky s vlastní volbou.

Místo, které nově zaškrtnete pod **Kopírováno do**, dostane každou položku, která není nastavena na Místní. Potvrzení uvádí počet položek a, je-li známa, kolik historie to představuje.

### Přímé repozitáře

Volba místa pod Jen mimo lokalitu, na které se doména už kopíruje, se jednou zeptá, pak vedle kopií vytvoří přímý repozitář, například `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, a nasměruje na něj položku. U cíle kopií bez místa otevře volba dialog s navrhovanou adresou a testem připojení, který nic nezaloží, a **Vytvořit a použít** repozitář vytvoří. Přímý repozitář přebírá klíč místa, třídu úložiště, limity, nastavení append-only a uchovávání a mění se s nimi. Když nový klíč místa repozitář neotevře, přímý repozitář si ponechá klíč, který má, a uložení to oznámí. Jeho snímky nesou značku `bv:direct` a každý další běh uchovávání je ponechá, takže přímý repozitář, který ztratil spojení se svým místem, nikdy nestárne podle místních pravidel. Klíč B2 omezený na jednu složku musí pokrývat adresu místa, nejen složku domény, jinak je složka vedle mimo dosah.

### Mimo objekt {#off-the-premises-mark}

Kopie se počítá jako samostatná lokalita, jen když je její místo v jiné lokalitě. Cloudové místo se počítá vždy a složka na tomto Unraidu nikdy; u NAS, rest-serveru nebo serveru SFTP odpovězte v podrobnostech místa na **Kde je zařízení?** volbou **Tady v domě** nebo **V jiné lokalitě**. Odpověď se počítá jen do lokalit a 3-2-1 na kartách a na Přehledu. Žádnou kopii nemění.

### Po opětovném sestavení

Volby kopírování žijí ve vlastním nastavení BombVaultu. Po opětovném sestavení přes Objevit zálohy bez obnoveného `/config` jsou pryč a kopírování všeho by znovu poslalo do B2 položky, které jste dřív vynechali. Replikace mimo lokalitu každé znovu sestavené domény se proto pozastaví. Přehled to zobrazí oranžově a řádek domény na kartě Domény nabídne **Potvrdit výchozí nastavení** s náhledem toho, co příští běh zkopíruje, a jmény v zálohách, které nemají žádnou položku, jež tam můžete vynechat. Pozastavení ukončí jen potvrzení; import souboru nastavení vrátí pravidla a výchozí hodnoty, ale pozastavení neukončí.

## Doména uložená na vzdáleném místě {#remote-primary-repositories}

Doména nemusí být uložena místně. Dokud její umístění záloh neobsahuje žádné zálohy, zvolte vzdálené místo pod **Uloženo v** na kartě Domény a doména bude zálohovat rovnou tam, bez místní kopie a bez kroku kopírování. Vzdálený repozitář je pak jedinou kopií, pokud se doména nekopíruje ještě na jiné místo. Každé vzdálené místo má stejné pojistky:

- **Test připojení** dřív, než se cokoli zapíše.
- **Limity šířky pásma** pro samotnou zálohu, tytéž přepínače `--limit-upload` a `--limit-download`, které používá kopie.
- **Ochrana append-only**, ověřená stejným aktivním testem odolnosti. Když je zapnutá, BombVault repozitář nikdy neprořezává, protože přihlašovací údaje na tomto stroji nesmějí být schopné smazat jedinou kopii zálohy.
- **Rozpočet růstu**, odvozený ze stejného trendu velikosti, který sleduje karta Úložiště.

Doména uložená na vzdáleném místě je zdrojem svých kopií stejně jako místní; viz [Kopie mezi místy s různými přihlašovacími údaji](storage-places.md#different-credentials).

!!! note "Přihlašovací údaje patří místu"
    Vzdálené místo má vlastní přihlašovací údaje. Místo nastavené se sdílenými přihlašovacími údaji ke cloudu je používá dál, dokud se jeho přístup nezmění v jeho podrobnostech.

## Neměnné (append-only) mimo lokalitu

Označte repozitář mimo lokalitu jako append-only, aby ransomware nebo kompromitovaný hostitel nemohl smazat nebo přepsat vaše zálohy. Druhá strana (`restic/rest-server` běžící v režimu `--append-only`) to **vynucuje**. BombVault to pouze **ověřuje** a nikdy nezobrazí zelenou jen na základě konfiguračního tvrzení.

Okno **Přidat místo** obsahuje recept připravený ke vložení pro rest-server v režimu append-only s jedním uživatelem pro tento BombVault. Na místě typu rest-server se zapnutým **Append-only** spustí **Otestovat append-only** v podrobnostech místa test odolnosti proti každé cestě domény, zapnuté kopii a repozitáři na místě a dá jednu odpověď za celé místo, takže append-only mimo lokalitu je dosažitelné bez ručního editování konfigurací.

!!! note "Úspěšné smazání v `/locks/` je očekávané"
    Append-only neznamená, že už nelze nic smazat. restic musí zakládat a uvolňovat vlastní zámky, proto `/locks/` záměrně zůstává zapisovatelný a smazatelný. Snapshoty a data za nimi, tedy přesně to, na co by mířil ransomware, odstranit nelze. Pokud si vzdálenou stranu ověříš sám, úspěšné smazání v `/locks/` je správné chování, ne díra v ochraně.

!!! warning "Neměnné repozitáře se z tohoto stroje nikdy neprořezávají"
    Neměnné mimo lokalitu záměrně nikdy neprořezává staré snímky. Nastavte pro něj **alarm rozpočtu růstu**, abyste byli upozorněni dříve, než se velikost repozitáře vymkne kontrole.

## Test odolnosti proti manipulaci

BombVault pravidelně dokazuje záruku append-only tím, že skutečně zkusí mazání proti repozitáři mimo lokalitu, zaměřené na neexistující objekt:

- **Odmítnuto** znamená chráněno.
- **Přijato** znamená nechráněno.
- **Neprůkazný** výsledek (server nedosažitelný, chyba autentizace) nikdy nezmění uložený verdikt.

Skutečný přechod z chráněno na nechráněno spustí jediné upozornění.

Na místě **Otestovat append-only** vyzkouší každou cestu domény, zapnutou kopii a repozitář s jejich vlastními přihlašovacími údaji a složí výsledky do jedné odpovědi: stačí jediný repozitář, který smazání přijme, a celé místo je *mazání přijato*.

## Cvičné obnovy po havárii

BombVault nabízí dvě úrovně důkazu, že vaše zálohy jsou skutečně obnovitelné, nejen přítomné.

- **Cvičné obnovy s ověřením (místní).** BombVault pravidelně spouští `restic check --read-data-subset` (omezené, nikdy plná obnova zaplňující disk) a zobrazuje odznak *naposledy ověřeno jako obnovitelné* na doménu. Kadence žije v Nastavení, Plány; odznak v Nastavení, Integrita.
- **Cvičné obnovy po havárii (mimo lokalitu).** BombVault obnoví skutečný cíl z repozitáře mimo lokalitu do jednorázového sandboxu, ověří jej soubor po souboru a bajt po bajtu, poté ukliďte. To dokazuje, že umíte obnovit z mimo lokalitu, nejen že repozitář odpovídá. Cvičí se jen místa v jiné lokalitě, protože kopie ve stejném domě nic nedokazuje pro případ ztráty domu. Doména kopírovaná na více z nich se při každém plánovaném běhu cvičí proti jednomu, postupně, a Přehled uvádí místo posledního cvičení.

**Vysvědčení ochrany proti ransomwaru** na Přehledu to shrne do zeleného / oranžového / červeného postoje na doménu, s kontrolním seznamem s věkovou značkou (mimo lokalitu nakonfigurováno, append-only ověřeno, replikace aktuální, cvičná obnova prošla, šifrování zapnuto, strategie prořezávání nastavena). Každý červený řádek odkazuje přímo na opravu a karta se rozsvítí zeleně jen na ověřených faktech.

## Řídicí panel příjemce (přijímací strana)

![Přijímající strana, sledovaná jen pro čtení, s kontrolou integrity na tomto stroji.](assets/screenshots/receiver.png)

*Přijímající strana, sledovaná jen pro čtení, s kontrolou integrity na tomto stroji.*

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

**1. Na VAULT postavte server v režimu append-only.** V BombVaultu na TOWER otevřete *Nastavení → Úložiště*, klikněte na **Přidat místo**, zvolte **rest-server** a klikněte na **Zobrazit recept**. Zkopírujte blok **Šablona pro Unraid**, uložte ho na VAULT jako `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, pak *Docker → Add Container* a vyberte **rest-server** ze seznamu šablon. Před spuštěním zapište zobrazený řádek `htpasswd` na VAULT do `/mnt/user/appdata/rest-server/.htpasswd`. Heslo se zobrazí jen jednou a nikdy se neukládá; recept ho i s uživatelem už vložil do formuláře na TOWER, takže to okno nechte otevřené. Řádek `htpasswd` nese stejné heslo, už zahašované bcryptem, takže sami nic hašovat nemusíte.

    Nechte `--append-only` v poli OPTIONS. Bez něj je VAULT zase jen obyčejné sdílení.

**2. Na TOWER přidejte místo.** Zadejte adresu VAULT, `http://VAULT:8000`, k uživateli a heslu, které vyplnil recept, a klikněte na **Otestovat připojení**. BombVault z nich sestaví adresu:

    rest:http://VAULT:8000/tower

První část cesty je uživatel htpasswd, zde `tower`, a každá doména pod ním dostane svou složku, například `rest:http://VAULT:8000/tower/container`. Na **Kde je zařízení?** odpovězte **V jiné lokalitě**, klikněte na **Přidat** a zaškrtněte místo pod **Kopírováno do** u domén, které tam mají jít.

**3. Na TOWER zapněte Append-only** v části **Ochrana** v podrobnostech místa a pak klikněte na **Otestovat append-only**. Test vyzkouší každou cestu domény, kopii a repozitář na místě a dá jednu odpověď za místo, která musí být *mazání odmítnuto*. Co odpovědi znamenají:

| Výsledek | Co se stalo |
| --- | --- |
| **mazání odmítnuto** | VAULT smazání odmítl. To je jediný vyhovující stav. |
| **mazání přijato** | VAULT smazání přijal. Chybí `--append-only`, nebo byl odebrán. |
| zpráva místo výsledku | Test nemohl proběhnout. Obvykle adresa není ta, kterou používá sám restic, nebo se změnily přihlašovací údaje. Nic se nezaznamená a nespustí se žádné upozornění. |

**4. Na VAULT sledujte, co přichází.** Zapněte *Nastavení → Příjemce*, otevřete kartu **Příjemce** a zaregistrujte repozitář jen pro čtení.

!!! warning "Umístění je cesta **uvnitř** kontejneru, zapsaná relativně k připojení hostitele"
    Zadejte `user/appdata/rest-server/tower/container`, **ne** `/mnt/user/appdata/…`. BombVault běží v kontejneru, kde je `/mnt` hostitele připojeno jinde; absolutní cesta hostitele tam neexistuje. Když ji vložíte, BombVault vám sdělí relativní cestu, kterou máte použít.

    **Odesílající APP_KEY** je klíč stroje TOWER, ne VAULT. Najdete jej na TOWER v *Nastavení → Systém*.

**5. Pokud chcete, udělejte to oboustranně.** Zopakujte stejných pět kroků opačným směrem: rest-server na TOWER přijímající kopii z VAULT. Každý stroj pak vynucuje neměnnost pro ten druhý a ani jeden nemůže smazat zálohy toho druhého.

## Řízená obnova

Vyhrazená záložka **Obnova** provede čistou nebo znovu sestavenou instalaci havarijním případem, na jednom místě:

1. **Zkontroluje, že BombVault umí číst vaše zálohy** (zádrhel se šifrovacím klíčem hned zkraje).
2. **Obnoví vlastní nastavení BombVaultu**, takže zálohovací cesty, cíle mimo lokalitu a přihlašovací údaje, které zbytek postupu potřebuje, přijdou předvyplněné. Zálohu nastavení čte z místa, které řádek Autozáloha uvádí pod **Uloženo v**, nebo z kopie Autozálohy pod **Kopírováno do**, a ukáže to místo s jeho adresou; chcete-li číst z jiného místa, nejdřív změňte řádek Autozáloha v kroku 3. Obnova se aplikuje přes sebe-restart přes Docker socket, takže se živá databáze nastavení nikdy nepřepisuje pod otevřeným handlem.
3. **Připojí vaše existující zálohy** přes řádky karty Domény: na řádku každé domény zvolte pod **Uloženo v** místo, kde leží její zálohy, a pod **Kopírováno do** místa s jejími kopiemi. Místo, které zatím žádný řádek nenabízí, třeba sdílenou složku, server nebo cloudový bucket, připojíte přes **Přidat místo**, stejné okno jako v Nastavení, Úložiště. **Připojit a zobrazit náhled** pak ověří, že zálohy jdou přečíst.
4. **Objeví** kontejnery, VM a sady souborů v něm uložené.
5. **Obnoví je všechny** (ponechané zastavené, takže je spustíte záměrně), s vaší sadou pro obnovu na jedno kliknutí.

!!! note "Kopie mimo lokalitu čekají po opětovném sestavení"
    Když krok 4 znovu sestaví položky bez starého nastavení, replikace mimo lokalitu těchto domén se pozastaví, dokud se nepotvrdí výchozí umístění. Viz [Umístění pro jednotlivé položky](#placement).

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
