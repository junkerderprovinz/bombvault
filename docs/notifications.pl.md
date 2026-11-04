# Powiadomienia

BombVault potrafi powiedzieć Ci, kiedy kopia się nie udała, kiedy się udała, kiedy jest spóźniona, i raz w tygodniu, jak poszło. Wszystko ustawia się w **Ustawienia, Powiadomienia**: co wyzwala wiadomość w karcie **Powiadomienia**, dokąd idą wiadomości w karcie **Kanały** i pingi meldunkowe w karcie **Healthchecks**.

![Ustawienia, Powiadomienia z wypełnionym kanałem Gotify.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Ustawienia, Powiadomienia z wypełnionym kanałem Gotify.](assets/screenshots/notifications/gotify-light.png#only-light)

## Co wysyła wiadomość {#what}

Karta **Powiadomienia** ustawia reguły dla wszystkich kanałów naraz:

- **Powiadom**: **Nigdy**, **Tylko przy niepowodzeniu** albo **Przy sukcesie i niepowodzeniu**, dla każdego kanału poniżej.
- **Podsumowuj zaplanowane przebiegi**: jedna wiadomość na zaplanowany przebieg, na przykład *42 z 45 się udało*, zamiast jednej na kontener lub maszynę wirtualną. Kopia uruchomiona ręcznie nadal wysyła własną wiadomość. Zamrożony gość, nieudane przechowywanie albo przekroczony budżet wzrostu wychodzą od razu, nawet przy włączonych podsumowaniach.
- **Powiadamiaj o aktualizacji kontenera**: jedna wiadomość dla każdego kontenera, który **Aktualizuj kontenery po udanej kopii zapasowej** przeniosło na nowszy obraz, żebyś mógł sprawdzić, czy nadal działa.
- **Zgłaszaj opóźnione kopie zapasowe**: raz dziennie o 09:00 BombVault sprawdza, czy włączona kopia jest spóźniona, czyli starsza niż dwukrotność jej harmonogramu, i wysyła jedną wiadomość na każdy przypadek. Następna udana kopia to czyści.
- **Powiadomienia Unraid**: wysyła do własnego systemu powiadomień Unraid, który może przekazywać dalej do Pushover, e-maila albo Discorda. Działa przez połączenie SSH z **Ustawienia, Integracje, SSH hosta**, więc ten klucz musi być autoryzowany. Na TrueNAS i innych hostach pozostaje wyłączone.
- **Cotygodniowe podsumowanie**: jedno podsumowanie tygodniowo z liczbą przebiegów, nowymi danymi kopii, informacją, czy kopie poza siedzibą są aktualne, i najczęstszymi błędami. Jego plan jest przyciskiem jak każdy inny plan, a **Wyłączone** go wyłącza. Zobacz [Harmonogramy](schedules.md#plans).

To, które anomalie wysyłają wiadomość, ustawia się w **Ustawienia, Weryfikacja**, zobacz [Anomalie](anomalies.md). Odroczona kopia wysyła jedną wiadomość, gdy zaczyna czekać, i jedną, gdy oczekiwanie się kończy, a nie jedną na każdą próbę.

**Wyślij test** na dole karty wysyła testową wiadomość przez każdy włączony kanał, z wartościami takimi, jakie stoją w formularzu.

## Kanały {#channels}

Każdy kanał ma własny przełącznik w karcie **Kanały**, a jego pola pojawiają się po włączeniu. Każdy kanał wysyła osobno, z własnym limitem czasu, więc jeden host, który nigdy nie odpowiada, nie wstrzymuje pozostałych, a wiadomość, której żaden kanał nie przyjął, jest próbowana ponownie.

| Kanał | Pola | Uwagi |
|---|---|---|
| **Webhook** | **Adres URL Webhook**, **Format Webhook** | Formaty: **Generic JSON**, **Discord**, **Slack**, **Gotify** i **ntfy**. Generic JSON wysyła `title`, `message` i `ok`. |
| **Apprise** | **URL API Apprise**, **Tagi (opcjonalnie)** | Wskaż punkt `/notify/<key>` swojego kontenera Apprise API, by dotrzeć do jego ponad 100 usług. Tagi wybierają podzbiór celów tego klucza. |
| **Matrix** | **Adres URL Homeserver**, **Token dostępu**, **ID pokoju** | Token zostaje na serwerze i nigdy więcej nie jest pokazywany. |
| **Gotify** | **URL serwera**, **Token aplikacji**, **Priorytet przy niepowodzeniu**, **Priorytet przy powodzeniu** | Priorytety biegną od 0 do 10 i zaczynają się od 8 dla niepowodzeń i 5 dla sukcesów. |
| **E-mail (SMTP)** | **Host SMTP**, **Port**, **Szyfrowanie**, **Nazwa użytkownika**, **Hasło**, **Adres nadawcy**, **Adres odbiorcy** | **Szyfrowanie** to **STARTTLS**, **TLS (implicit)** albo **Brak**. |

Sekrety, takie jak token Matrix, token aplikacji Gotify i hasło SMTP, są przechowywane zaszyfrowane. Pod zapisanym sekretem pole mówi **Zapisano. Zostaw pole puste, aby to zachować.** Gdy zmieniasz adres serwera, BombVault nie wysyła starego sekretu w nowe miejsce: pole prosi o niego ponownie, a nowy adres zapisuje się, gdy go wpiszesz.

## Healthchecks {#healthchecks}

Usługa taka jak [Healthchecks.io](https://healthchecks.io) podnosi alarm, gdy BombVault przestaje się meldować. Wklej **Adres URL pingu** tej kontroli, a BombVault będzie ją pingował przez cały cykl życia każdej kopii: na początku, a potem po sukcesie albo z `/fail`. Robi to zawsze, gdy adres jest ustawiony, bez względu na **Powiadom**, więc kontrola pozostaje zielona po sukcesie nawet przy wiadomościach tylko o niepowodzeniach.

Sekcja może pingować własny adres: **Wyjątek** w karcie dodaje taki, a linia mówi **Wszystkie pozostałe sekcje pingują ten adres URL.**

## Przykład: Gotify w telefonie {#example-gotify}

Prowadzisz Gotify pod `https://gotify.example.com` i chcesz, żeby błędy dzwoniły w telefonie, a sukcesy przychodziły po cichu.

1. W Gotify otwórz **Apps**, utwórz aplikację o nazwie `BombVault` i skopiuj jej token.
2. W BombVault otwórz **Ustawienia, Powiadomienia**. W **Powiadomienia** ustaw **Powiadom** na **Przy sukcesie i niepowodzeniu** i włącz **Podsumowuj zaplanowane przebiegi**, żeby noc z 45 kopiami wysłała jedną wiadomość zamiast 45.
3. W **Kanały** włącz **Gotify**.
4. Wpisz `https://gotify.example.com` jako **URL serwera**. BombVault sam dodaje `/message`.
5. Wklej token do **Token aplikacji**.
6. Zostaw **Priorytet przy niepowodzeniu** na 8 i **Priorytet przy powodzeniu** na 5. Aplikacja Gotify na Androida traktuje 8 i więcej jako wysoki priorytet, więc dźwięk robią tylko błędy.
7. Naciśnij **Wyślij test**. Linia pod kartą mówi, czy Gotify go przyjął, a odrzucony token jest nazwany po imieniu.

Następny zaplanowany przebieg wysyła jedno podsumowanie z priorytetem 5, albo 8, jeśli cokolwiek w nim się nie udało.

## Przykład: ntfy jako webhook {#example-ntfy}

ntfy nie potrzebuje konta dla publicznego tematu. Wybierz nazwę tematu trudną do odgadnięcia, bo każdy, kto ją zna, może go czytać.

1. W **Kanały** włącz **Webhook**.
2. Wpisz adres tematu jako **Adres URL Webhook**, na przykład `https://ntfy.sh/bombvault-tower-7f3k`, albo adres własnego serwera ntfy.
3. Ustaw **Format Webhook** na **ntfy**.
4. Zasubskrybuj ten sam temat w aplikacji ntfy i naciśnij **Wyślij test**.

BombVault wysyła wiadomość jako treść z tytułem w `X-Title`. Błąd wychodzi z wysokim priorytetem i tagiem `warning`, więc wyróżnia się w aplikacji.

## Gdy wiadomości nie docierają {#troubleshooting}

Zobacz [Powiadomienia nie docierają](troubleshooting.md#notifications-do-not-arrive) w Rozwiązywaniu problemów.
