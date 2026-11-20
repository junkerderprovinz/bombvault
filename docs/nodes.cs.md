# Uzly

Uzel je BombVault bez webového rozhraní, třeba na VPS nebo na malém serveru u kamaráda. Zálohuje kontejnery a složky na vlastním serveru a BombVault, na kterém ho přidáš, na této stránce nazývaný **domov**, mu říká, kdy zálohovat a kam zálohy posílat. Staráš se o něj z webového rozhraní domova, vedle vlastních položek.

![Karta uzlu vps-falkenstein na stránce Instance: Připojeno, jeho verze, jeho zálohy na přijímacím serveru, kdy se zálohuje která sekce a k tomu Změnit cíl, Nahradit a Uvolnit.](assets/screenshots/nodes/card-dark.png#only-dark)
![Karta uzlu vps-falkenstein na stránce Instance: Připojeno, jeho verze, jeho zálohy na přijímacím serveru, kdy se zálohuje která sekce a k tomu Změnit cíl, Nahradit a Uvolnit.](assets/screenshots/nodes/card-light.png#only-light)

## Jak to funguje {#how-it-works}

- **Domov drží klíč uzlu.** Když přidáš uzel, domov mu vytvoří `APP_KEY`, ponechá si jeho zapečetěnou kopii a jednou ti ho předá v souboru compose uzlu. S touto kopií domov na uzel dosáhne a otevře jeho zálohy.
- **Uzel nepotřebuje otevřený port.** Domov a uzel spolu mluví přes přeposílač vlastním soukromým spojením, odděleně od tvé skupiny párování. Domov každý požadavek zahajuje a podepisuje a uzel po domovu nechce nic kromě svého pozdravu. Jediný cíl, který otevřený port potřebuje, je [ponechání záloh na uzlu](#keep-on-node).
- **Zálohy nikdy nejdou přes přeposílač.** Uzel je posílá rovnou tam, kam vybereš: na přijímací server domova, do některého z cílů domova nebo na rest-server přímo na uzlu.
- **Uzel sám nic neplánuje.** Nespouští žádné plány, neposílá oznámení a nepřipojuje se k žádné skupině ani pevnosti. Domov spouští jeho zálohy, hlídá, jak probíhají, a posílá upozornění.
- **Uzel zálohuje kontejnery a sady složek.** VM, flashka, datové sady ZFS a autozáloha zůstávají plným instalacím.

Spojení používá vlastní přeposílač domova, když ho jeho skupina má, a jinak přeposílač projektu, i když se domov páruje bez přeposílače.

## Než začneš {#before-you-start}

- **Přihlašovací heslo na domově.** Přidání uzlu, opětovné zobrazení jeho souboru compose a otevírání jeho položek vyžadují přihlášení heslem, viz [Přihlášení](configuration.md#login).
- **Docker s Compose na serveru uzlu.**
- **Místo pro zálohy**, jedno ze tří v části [Kam jdou zálohy](#placement). Nejbezpečnější volbou je přijímací server, protože tam uzel nemůže nic smazat.

## Přidání uzlu {#add}

1. Na domově otevři **Instance** a vpravo dole zvol **Přidat uzel**.
2. **Název.** Napiš, jak se má uzel tady zobrazovat, třeba podle názvu serveru, a zvol **Další**.

    ![Okno Přidat uzel v prvním kroku s napsaným názvem vps-helsinki.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![Okno Přidat uzel v prvním kroku s napsaným názvem vps-helsinki.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Zálohy.** Vyber, kam zálohy půjdou, a vyplň, na co se to místo ptá. **Zálohuje denně v** je čas, kdy začnou obě sekce. Domov navrhne půl hodiny po svých vlastních zálohách kontejnerů, aby obě nesahaly na stejné disky naráz. Později můžeš každé sekci dát na kartě vlastní plán. Zvol **Další**.

    ![Druhý krok: vybraný přijímací server, Adresa pro uzly vyplněná adresou https a Zálohuje denně v 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![Druhý krok: vybraný přijímací server, Adresa pro uzly vyplněná adresou https a Zálohuje denně v 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Spuštění.** Okno ukáže soubor compose uzlu s vyplněnými `MODE`, `APP_KEY`, `NODE_HOME` a `NODE_NAME`. Zkopíruj ho a `APP_KEY` si dobře uschovej. Na serveru uzlu:

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # vlož soubor z okna
    docker compose up -d
    ```

    Než ho spustíš, nasměruj svazek Host Data na data, která chceš zálohovat, a nastav `HOST_SOURCE_ROOT` na stejnou cestu, jak ukazuje příklad níže.

5. Okno čeká na uzel. Jakmile uzel odpoví, domov mu řekne, kam jdou jeho zálohy, a ověří, že na ně dosáhne. Okno pak píše **vps-helsinki je online a přijímá pokyny od této instance.**

    ![Poslední krok: soubor compose, potom uzel online a jeho zálohy umístěné na přijímacím serveru.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![Poslední krok: soubor compose, potom uzel online a jeho zálohy umístěné na přijímacím serveru.](assets/screenshots/nodes/add-start-light.png#only-light)

Okno můžeš zavřít, ještě než uzel odpoví. Jeho karta pak říká **Čeká se na uzel** a domov mu sdělí, kam jdou jeho zálohy, jakmile se objeví.

### Příklad: VPS, který zálohuje na přijímací server {#example-vps}

Domov je krabice s Unraidem jménem `tower`, na které běží [přijímací server](offsite-recovery.md#receiving-server). Reverzní proxy ho zveřejňuje jako `https://backup.tower.example.org`. Na VPS `vps-helsinki` běží Nextcloud a Caddy s daty pod `/srv/appdata`.

1. Na `tower`: **Instance**, **Přidat uzel**, název `vps-helsinki`, **Další**.
2. Vyber **Přijímací server této instance** a do **Adresa pro uzly** napiš `https://backup.tower.example.org`. Kontejnery domova se zálohují ve 02:30, takže okno nabídne 03:00. Nech to tak a zvol **Další**.
3. Na VPS ulož soubor z okna se svazky a `HOST_SOURCE_ROOT` upravenými pro `/srv/appdata`:

    ```yaml
    services:
      bombvault-node:
        image: junkerderprovinz/bombvault:latest
        restart: unless-stopped
        hostname: bombvault-node
        container_name: bombvault-node
        volumes:
          - ./config:/config
          - /var/run/docker.sock:/var/run/docker.sock
          - /srv/appdata:/host/user
        environment:
          MODE: node
          APP_KEY: "<filled in by home>"
          NODE_HOME: "<filled in by home>"
          NODE_NAME: "vps-helsinki"
          PLATFORM: generic
          HOST_SOURCE_ROOT: /srv/appdata
    ```

4. `docker compose up -d`. Za pár sekund okno na `tower` napíše, že je uzel online, a **Zálohy jdou sem: https://backup.tower.example.org.**
5. Ve 03:00 domov spustí zálohy uzlu. Druhý den ráno ukáže karta **Uzly** v Přehledu poslední zálohu `vps-helsinki` a **Zálohy** vypíšou jeho kontejnery ve vlastní sekci.

Jak se data kontejneru najdou na obyčejném hostiteli s Dockerem, popisuje [Rozpoznávání zdrojů zálohy](configuration.md#backup-source-detection).

## Kam jdou zálohy {#placement}

Každý uzel má pro obě své sekce jeden cíl. **Změnit cíl** na jeho kartě ho změní, jakmile je uzel online. Nové místo dostane nové repozitáře a zálohy na starém místě zůstanou, kde jsou.

### Přijímací server této instance {#to-receiver}

Uzel zapisuje na přijímací server domova v režimu append-only: zálohy přidávat může, ale žádnou nesmaže. Dostane tam vlastní přihlášení pojmenované podle uzlu a jeden repozitář na sekci ve vlastní složce. Domov přidá do každého repozitáře svůj klíč a sám je uklízí, protože uzel to nedokáže.

Uzel se k serveru dostane přes **Adresa pro uzly**, kterou nastavíš ve druhém kroku nebo v okně **Přijímací server**. Uzel obvykle posílá z internetu, takže adresa musí být jedna z těchto:

- `https://`, například za reverzní proxy, Pangolinem nebo tunelem. Když server ukazuje vlastní certifikát místo certifikátu od veřejné autority, vlož ho ve formátu PEM do **Certifikát**.
- `http://` se soukromou IP adresou, například `10.0.0.5` na spojení WireGuard nebo `100.101.102.103` v Tailscale.

Holé `http://` na veřejnou adresu se odmítne, protože by uzel posílal zálohy přes internet nešifrovaně. Když adresu změníš, domov to oznámí každému uzlu na přijímači a uzel, který je zrovna pryč, se to dozví, až se vrátí.

### Cíl {#to-location}

Uzel zapisuje do některého z cílů domova, na který sám dosáhne: do služby S3 nebo na REST server, přes `https://` nebo na soukromé adrese. Jeho repozitáře tam leží v `bombvault-nodes/<node id>/containers` a `bombvault-nodes/<node id>/files`.

Uzel se přihlašuje [uloženými přihlašovacími údaji](offsite-recovery.md#sign-ins) vytvořenými pro něj, které se musí lišit od těch, kterými do cíle zapisuje domov. Pokud okno říká, že žádné nejsou, nejdřív je přidej. S těmito údaji může uzel v cíli obvykle mazat, co tam leží, na rozdíl od přijímacího serveru, a okno na to upozorní.

### Nechat je na uzlu {#keep-on-node}

Uzel si zálohy drží sám, za vlastním rest-serverem v režimu append-only, a domov si z každé sekce stáhne kopii do svého úložiště. Hodí se to pro uzel, kterému domov nemůže nabídnout místo. Je to jediný cíl, který potřebuje otevřený port.

![Druhý krok s vybraným Nechat je na uzlu, adresou https://203.0.113.7:8443 a poznámkou o otevřeném portu.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![Druhý krok s vybraným Nechat je na uzlu, adresou https://203.0.113.7:8443 a poznámkou o otevřeném portu.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. Do **Adresa uzlu** napiš, kde domov uzel zastihne: jeho veřejnou adresu a volný port, například `https://203.0.113.7:8443`, bez složky za ním. `http://` použij jen se soukromou adresou, jako výše.
2. Uzel na tom portu spustí kontejner `bombvault-node-rest-server` s repozitáři ve složce `bombvault-node` pod Host Data. U `https://` vytvoří pro tu adresu certifikát a domov si ho připne.
3. Povol port ve firewallu uzlu, aby se k němu domov dostal. Port, který na uzlu drží jiný kontejner, se odmítne i se jménem toho kontejneru.

Domov stáhne sekci hned, jak na uzlu skončí její zálohy, a záložně každou hodinu ve čtvrt, i se vypnutým **Stažení**. Co domov stáhl, zůstává u domova jako každá [stažená kopie](offsite-recovery.md#receiver), i když uzel zmizí. Uzel si vlastní repozitáře uklízí, když ho o to domov požádá, protože jeho rest-server mazání nepřijímá, a jeho sady složek vynechávají složku `bombvault-node`, takže nikdy nezálohuje vlastní zálohy.

## Kdy uzly zálohují {#schedule}

Plán každého uzlu patří domovu. Karta ukazuje plán pro každou sekci, například **Kontejnery** *denně ve 2:00* a **Složky** *týdně (ne) ve 4:00*. Klik otevře stejný editor jako v [Plány](schedules.md#plans) a změna se uloží hned.

![Plán kontejnerů uzlu otevřený z jeho karty, s Vypnuto, Denně, Týdně, Každých N dní a Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![Plán kontejnerů uzlu otevřený z jeho karty, s Vypnuto, Denně, Týdně, Každých N dní a Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **Uzel, který byl pryč**, spustí zmeškané zálohy, jakmile se vrátí. Domov se dívá každých pět minut.
- **Zaneprázdněného uzlu** se domov zeptá znovu za pět minut.
- **Zatímco domov uklízí** staré zálohy uzlu, uzel nic nespustí, dokud domov neskončí. Na přijímacím serveru a na samotném uzlu se toto okno otevírá každou neděli ve 03:00 a trvá nejvýš šest hodin. V cíli se řídí plánem kopií mimo lokalitu, který má domov pro ten cíl. Domov uchovává zálohy podle vlastního pravidla sekce v **Jak dlouho uchovávat**, nebo podle pravidla cíle, pokud nějaké má.
- **V seznamu běhů uzlu** se zálohy a obnovy, které spustil domov, ukazují jako spuštěné domovem.

## Hlídání uzlů {#watch}

Karta **Uzly** v Přehledu vypisuje každý uzel s jeho stavem, poslední zálohou, chybami a otevřenými anomáliemi. Objeví se, jakmile máš nějaký uzel.

![Karta Uzly v Přehledu se třemi uzly: jeden připojený, jeden neaktivní se dvěma chybami a jeden stále čekající na uzel.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![Karta Uzly v Přehledu se třemi uzly: jeden připojený, jeden neaktivní se dvěma chybami a jeden stále čekající na uzel.](assets/screenshots/nodes/overview-card-light.png#only-light)

Domov pošle přes své [kanály oznámení](notifications.md) jedno upozornění na epizodu a další až po skončení té epizody:

- **Záloha selhala**, například *BombVault: vps-helsinki: backup failed*, s položkou a důvodem. Epizodu ukončí pozdější záloha té položky, která projde.
- **Uzel je mimo dosah**, například *BombVault: vps-helsinki is out of reach*. Domov počká tak dlouho, kolik činí nejkratší spínač mrtvého muže na repozitářích uzlu, tedy 26 hodin, pokud to nezměníš, nebo 6 hodin, dokud uzel ještě nemá cíl.

Sledování přijímače na domově hlídá i každý repozitář uzlu, se spínačem mrtvého muže a denním `restic check` ve 04:00, viz [Přijímač a stahovač](offsite-recovery.md#receiver).

## Položky uzlu {#entries}

V **Zálohy** mají seznamy Kontejnery a Složky pod tvými vlastními položkami sekci pro každý uzel. Vlastní kontejner uzlu je vynechaný. **Zálohovat nyní** v nabídce položky spustí její zálohu na uzlu a **Otevřít** otevře její stránku.

V sekci uzlu v seznamu Složky přidá **Přidat sadu složek** sadu složek na uzlu a **Upravit** v nabídce sady ji změní. Obojí otevře stejné okno jako u tvých vlastních sad. Výběr složek v něm prochází složky uzlu, ale nové tam nevytváří.

![Sekce uzlu vps-falkenstein v seznamu Kontejnery: nextcloud, nextcloud-db, caddy a uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![Sekce uzlu vps-falkenstein v seznamu Kontejnery: nextcloud, nextcloud-db, caddy a uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

Stránka položky uzlu říká **Na uzlu** a jméno uzlu a vypisuje její zálohy. **Obnovit** u jedné z nich obnoví položku na uzlu a nahradí to, co tam je teď. Kontejner se kvůli tomu zastaví, stejně jako na domově.

![Stránka nextcloud na vps-falkenstein s jeho zálohami a tlačítkem Obnovit u každé.](assets/screenshots/nodes/entry-dark.png#only-dark)
![Stránka nextcloud na vps-falkenstein s jeho zálohami a tlačítkem Obnovit u každé.](assets/screenshots/nodes/entry-light.png#only-light)

Přes domov dosáhneš na pevný seznam vlastních cest uzlu: jeho kontejnery, sady složek, běhy a anomálie a jejich zálohování a obnovu. Nastavení, klíče a úložiště uzlu zůstávají mimo dosah a nic na tom seznamu nic nemaže. Viz [Uzly v API](api.md#nodes).

## Nahrazení a uvolnění {#replace-release}

**Nahradit** ukáže soubor compose uzlu znovu, po přihlášení heslem. Použij to, když chceš uzel přesunout na jiný server nebo ho znovu postavit: spusť soubor tam a starý server vypni. Stejný `APP_KEY` a `NODE_HOME` z něj udělají zase tentýž uzel, s jeho zálohami a historií. Dva servery se stejným `APP_KEY` se počítají jako jeden uzel, takže nikdy neprovozuj oba.

**Uvolnit** uzel propustí. Přestane přijímat pokyny od domova a už nic nezálohuje. Jeho zálohy zůstanou, kde jsou, stejně jako každá kopie, kterou domov stáhl. Jeho přihlášení na přijímacím serveru zmizí a domov přestane hlídat jeho repozitáře. Uzel, který je pryč, když ho uvolníš, si ponechá kartu jako **Uvolňuje se**, dokud se to nedozví. Uvolněný uzel už se stejným `APP_KEY` žádný domov nepřijme. Chceš-li server použít znovu, přidej ho jako nový uzel a spusť ho s novým souborem compose a prázdnou složkou `config`.

## Zabezpečení {#security}

Pokud někdo převezme domov, ovládá i jeho uzly, a uzel drží Docker svého serveru, což je tam root. Chraň domov heslem a drž ho mimo internet. Pokud někdo převezme uzel, nezíská skupinu párování, pevnost, žádný jiný uzel ani jiný repozitář a na přijímacím serveru nemůže smazat historii. Podrobnosti jsou v [Uzly v bezpečnostním modelu](configuration.md#nodes).

[Sada pro obnovu](offsite-recovery.md#recovery-kit) má kapitolu **Uzly** s `APP_KEY` každého uzlu, jeho heslem resticu a jeho repozitáři, takže uzel nebo jeho zálohy obnovíš i holým resticem.
