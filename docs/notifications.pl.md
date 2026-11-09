# Powiadomienia

BombVault potrafi powiedzieć Ci, kiedy kopia się nie udała, kiedy się udała, kiedy jest spóźniona, i raz w tygodniu, jak poszło. Wszystko ustawia się w **Ustawienia, Powiadomienia**: co BombVault wysyła poza wiadomościami o pojedynczych kopiach w karcie **Powiadomienia**, a dokąd idą wiadomości w karcie **Kanały**. Każdy kanał sam decyduje, kiedy się odezwać, a kontrola Healthchecks też jest kanałem.

![Ustawienia, Powiadomienia z wypełnionym kanałem Gotify.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Ustawienia, Powiadomienia z wypełnionym kanałem Gotify.](assets/screenshots/notifications/gotify-light.png#only-light)

## Co wysyła wiadomość {#what}

Przełączniki w karcie **Powiadomienia** dotyczą wszystkich kanałów:

- **Podsumowuj zaplanowane przebiegi**: jedna wiadomość na zaplanowany przebieg, na przykład *42 z 45 się udało*, zamiast jednej na kontener lub maszynę wirtualną. Kopia uruchomiona ręcznie nadal wysyła własną wiadomość. Zamrożony gość, nieudane przechowywanie albo przekroczony budżet wzrostu wychodzą od razu, nawet przy włączonych podsumowaniach.
- **Powiadamiaj o aktualizacji kontenera**: jedna wiadomość dla każdego kontenera, który **Aktualizuj kontenery po udanej kopii zapasowej** przeniosło na nowszy obraz, żebyś mógł sprawdzić, czy nadal działa.
- **Zgłaszaj opóźnione kopie zapasowe**: raz dziennie o 09:00 BombVault sprawdza, czy włączona kopia jest spóźniona, czyli starsza niż dwukrotność jej harmonogramu, i wysyła jedną wiadomość na każdy przypadek, z nazwą elementu, który został w tyle. Następna udana kopia to czyści.
- **Cotygodniowe podsumowanie**: jedno podsumowanie tygodniowo z liczbą przebiegów, nowymi danymi kopii, spóźnionymi sekcjami, informacją, czy kopie poza siedzibą są aktualne, i najczęstszymi błędami. Jego plan jest przyciskiem jak każdy inny plan, a **Wyłączone** go wyłącza. Zobacz [Harmonogramy](schedules.md#plans).

To, czy kanał słyszy też o sukcesach, ustawia się przy samym kanale, w **Powiadom**. To, które anomalie wysyłają wiadomość, ustawia się w **Ustawienia, Weryfikacja**, zobacz [Anomalie](anomalies.md). Odroczona kopia wysyła jedną wiadomość, gdy zaczyna czekać, i jedną, gdy oczekiwanie się kończy, a nie jedną na każdą próbę.

## Kanały {#channels}

**Dodaj kanał** na dole karty **Kanały** oferuje **Powiadomienia Unraid**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **E-mail (SMTP)**, **Apprise**, **Webhook** i **Healthchecks**. Dodaj ich tyle, ile chcesz, także dwa tego samego rodzaju, na przykład jeden temat ntfy dla niepowodzeń i drugi dla wszystkiego. Nowy kanał startuje wyłączony, więc zapisuje się, zanim wypełnisz jego pola.

Każdy kanał ma:

- własny przełącznik, który go wyłącza bez utraty jego pól.
- **Powiadom**: **Nigdy**, **Tylko przy niepowodzeniu** albo **Przy sukcesie i niepowodzeniu**. Nowy kanał zaczyna od **Tylko przy niepowodzeniu**. Kopia, która się udała, ale coś pominęła, trafia też do kanału ustawionego tylko na niepowodzenia.
- **Nazwa**, która jest opcjonalna i pojawia się zamiast rodzaju. Przydaje się, gdy masz dwa kanały tego samego rodzaju.
- **Wyślij test**, który wysyła testową wiadomość przez ten kanał z wartościami takimi, jakie stoją w formularzu, zapisanymi czy nie. Linia pod kanałem mówi, czy usługa ją przyjęła, a jeśli nie, to dlaczego.
- **Usuń kanał**, kosz obok.

Każdy kanał wysyła osobno, z własnym limitem czasu, więc jeden host, który nigdy nie odpowiada, nie wstrzymuje pozostałych. Gdy usługa zawiedzie tak, że kolejna próba mogłaby pomóc, kanał próbuje do trzech razy. Alert o opóźnieniu, budżecie albo anomalii, którego żaden kanał nie przyjął, jest później wysyłany ponownie, tak samo jak cotygodniowe podsumowanie.

| Kanał | Pola | Uwagi |
|---|---|---|
| **Powiadomienia Unraid** | brak | Wysyła do własnego systemu powiadomień Unraid, który może przekazywać dalej do Pushover, e-maila albo Discorda. Działa przez połączenie SSH z **Ustawienia, Integracje, SSH hosta**, więc ten klucz musi być autoryzowany. Na TrueNAS i innych hostach milczy, a kanał mówi dlaczego. |
| **ntfy** | **Adres URL tematu**, **Token dostępu (opcjonalnie)** | Adres kończy się Twoim tematem. Token jest potrzebny tylko dla chronionego tematu. |
| **Gotify** | **URL serwera**, **Token aplikacji**, **Priorytet przy niepowodzeniu**, **Priorytet przy powodzeniu** | Priorytety biegną od 0 do 10 i zaczynają się od 8 dla niepowodzeń i 5 dla sukcesów. |
| **Discord** | **Adres URL Webhook** | Adres webhooka z ustawień kanału na Discordzie. |
| **Slack** | **Adres URL Webhook** | Adres przychodzącego webhooka Twojej aplikacji Slack. |
| **Matrix** | **Adres URL Homeserver**, **Token dostępu**, **ID pokoju** | Konto, do którego należy token, musi być w pokoju. |
| **E-mail (SMTP)** | **Host SMTP**, **Port**, **Szyfrowanie**, **Nazwa użytkownika**, **Hasło**, **Adres nadawcy**, **Adres odbiorcy** | **Szyfrowanie** to **STARTTLS**, **TLS (implicit)** albo **None**. |
| **Apprise** | **URL API Apprise**, **Tagi (opcjonalnie)**, **Adresy Apprise (opcjonalnie)** | Wskaż punkt `/notify/<key>` swojego kontenera Apprise API, by dotrzeć do jego ponad 100 usług, a tagi wybiorą podzbiór celów tego klucza. Albo wpisz adresy Apprise, jeden na linię, na przykład `tgram://` dla Telegrama albo `pover://` dla Pushover, a adres API zakończ na `/notify` bez klucza. |
| **Webhook** | **Adres URL Webhook**, **Metoda**, **Nagłówki**, **Treść** | Żądanie, które składasz sam, zobacz [Webhook](#webhook). |
| **Healthchecks** | **Adres URL pingu** | Zobacz [Healthchecks](#healthchecks). |

Sekrety, takie jak tokeny, hasło SMTP i wartości nagłówków webhooka, są przechowywane zaszyfrowane, a serwer nigdy nie oddaje ich stronie. Pod zapisanym sekretem pole mówi **Zapisano. Zostaw pole puste, aby to zachować.** Gdy zmieniasz, dokąd kanał wysyła, BombVault nie wysyła starego sekretu w nowe miejsce: pole mówi **Nowy adres zapisze się, gdy wpiszesz to ponownie.**, a nowy adres zapisuje się, gdy wpiszesz sekret.

Ustawienia z wersji sprzed listy kanałów są przenoszone przy pierwszym starcie i nic nie ginie. Każdy kanał, który miałeś wypełniony, staje się osobnym wpisem z Twoim dotychczasowym wyborem w **Powiadom**. Webhook ustawiony na format Discord, Slack, Gotify albo ntfy staje się kanałem tego rodzaju, a webhook Generic JSON dalej wysyła `title`, `message` i `ok` przez swoją **Treść**. Przełącznik Unraid staje się kanałem Unraid, a adres Healthchecks razem z wyjątkami kanałem Healthchecks.

### Webhook {#webhook}

Kanał **Webhook** wysyła jedno żądanie HTTP pod dowolny adres. **Metoda** to GET, POST albo PUT. **Dodaj nagłówek** dodaje wiersz w **Nagłówki**, a wartości nagłówków są przechowywane jako sekrety, więc nagłówek `Authorization` jest tam bezpieczny. **Treść** jest wysyłana dokładnie tak, jak ją napiszesz, z wypełnionymi tymi symbolami zastępczymi:

| Symbol zastępczy | Zamienia się na |
|---|---|
| `%%title%%` | tytuł wiadomości |
| `%%body%%` | samą wiadomość |
| `%%ok%%` | `true`, a przy niepowodzeniu `false` |
| `%%event%%` | to, co się stało, na przykład `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` albo `critical` |
| `%%importance%%` | odpowiadający mu poziom w Unraid: `normal`, `warning` albo `alert` |
| `%%link%%` | link należący do wiadomości albo pusty tekst, gdy go nie ma |
| `%%key%%` | sekcję, na przykład `container` albo `VM` |
| `%%time%%` | czas w UTC, na przykład `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

Symbole zastępcze działają też w adresie i w wartościach nagłówków, a każda wartość jest escapowana stosownie do miejsca, w którym stoi: jako JSON, gdy nagłówek `Content-Type` wskazuje JSON, i jako wartość zapytania w adresie. Nowy webhook startuje z tym nagłówkiem i wysyła `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}`. Nazwa, której BombVault nie zna, zostaje tak, jak jest napisana, więc literówkę zobaczysz w wiadomości. Zostaw **Treść** pustą, by nie wysyłać żadnej.

## Healthchecks {#healthchecks}

Usługa taka jak [Healthchecks.io](https://healthchecks.io) podnosi alarm, gdy BombVault przestaje się meldować. Dodaj kanał **Healthchecks** i wklej **Adres URL pingu** tej kontroli, a BombVault będzie ją pingował przez cały cykl życia każdej kopii: na początku, a potem po sukcesie albo z `/fail`. Ten kanał nie ma wyboru **Powiadom**, bo kontrola potrzebuje pingów o sukcesie, żeby pozostać zielona. Sekcja, która nie ma dokąd robić kopii, liczy się jako nieudana, więc jej kontrola robi się czerwona, zamiast dostać sukces.

Sekcje z tym samym adresem dzielą jedną kontrolę. Gdy kopia jednej z nich się nie powiedzie, udane kopie pozostałych też pingują `/fail` i ją wymieniają, więc kontrola zostaje czerwona, dopóki ten element znów nie będzie miał udanej kopii albo dopóki nie potwierdzisz niepowodzenia w **Wymaga uwagi**.

Sekcja może pingować własny adres: **Wyjątek** w kanale Healthchecks dodaje taki, a linia mówi wtedy **Wszystkie pozostałe sekcje pingują ten adres URL.** **Wyślij test** zapisuje wpis w dzienniku każdej kontroli, co nie zmienia jej stanu.

## Przykład: Gotify w telefonie {#example-gotify}

Prowadzisz Gotify pod `https://gotify.example.com` i chcesz, żeby błędy dzwoniły w telefonie, a sukcesy przychodziły po cichu.

1. W Gotify otwórz **Apps**, utwórz aplikację o nazwie `BombVault` i skopiuj jej token.
2. W BombVault otwórz **Ustawienia, Powiadomienia** i włącz **Podsumowuj zaplanowane przebiegi**, żeby noc z 45 kopiami wysłała jedną wiadomość zamiast 45.
3. W **Kanały** naciśnij **Dodaj kanał** i wybierz **Gotify**.
4. Wpisz `https://gotify.example.com` jako **URL serwera**. BombVault sam dodaje `/message`.
5. Wklej token do **Token aplikacji**.
6. Ustaw **Powiadom** na **Przy sukcesie i niepowodzeniu**. Zostaw **Priorytet przy niepowodzeniu** na 8 i **Priorytet przy powodzeniu** na 5. Aplikacja Gotify na Androida traktuje 8 i więcej jako wysoki priorytet, więc dźwięk robią tylko błędy.
7. Włącz kanał i naciśnij **Wyślij test**. Linia pod kanałem mówi, czy Gotify go przyjął, a odrzucony token jest nazwany po imieniu.

Następny zaplanowany przebieg wysyła jedno podsumowanie z priorytetem 5, albo 8, jeśli cokolwiek w nim się nie udało.

## Przykład: ntfy bez konta {#example-ntfy}

ntfy nie potrzebuje konta dla publicznego tematu. Wybierz nazwę tematu trudną do odgadnięcia, bo każdy, kto ją zna, może go czytać.

1. W **Kanały** naciśnij **Dodaj kanał** i wybierz **ntfy**.
2. Wpisz adres tematu jako **Adres URL tematu**, na przykład `https://ntfy.sh/bombvault-tower-7f3k`, albo temat na własnym serwerze ntfy. Przy publicznym temacie zostaw **Token dostępu (opcjonalnie)** pusty.
3. Zasubskrybuj ten sam temat w aplikacji ntfy, włącz kanał i naciśnij **Wyślij test**.

BombVault wysyła wiadomość jako treść z tytułem w nagłówku `Title`. Błąd wychodzi z wysokim priorytetem i tagiem `warning`, więc wyróżnia się w aplikacji, a wiadomość o maszynie wirtualnej, która została zamrożona, wychodzi jako pilna.

## Gdy wiadomości nie docierają {#troubleshooting}

Zobacz [Powiadomienia nie docierają](troubleshooting.md#notifications-do-not-arrive) w Rozwiązywaniu problemów.
