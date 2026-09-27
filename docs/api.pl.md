# API i integracje

BombVault ma małe API HTTP dla skryptów, pulpitów i automatyki domowej. Odczytuje to samo, co pokazuje pulpit, i może uruchomić kopię. Wszystko inne, jak przywracanie, usuwanie kopii i ustawienia, zostaje w interfejsie WWW.

## Tokeny {#tokens}

Każde żądanie potrzebuje tokenu API, nawet gdy nie ustawiono hasła logowania. Utwórz go w **Ustawienia, System, Tokeny API**:

1. Wpisz nazwę, która mówi, gdzie token jest używany, na przykład „Home Assistant” albo „Uptime Kuma”.
2. Włącz **Pozwól uruchamiać kopie**, jeśli token ma uruchamiać kopie. Bez tego może tylko czytać.
3. Kliknij **Utwórz token**. Token pokazuje się tylko raz. BombVault przechowuje jedynie jego odcisk, więc skopiuj go teraz.

Wysyłaj token w nagłówku: `Authorization: Bearer <token>` albo `X-API-Key: <token>`. Token zaczyna się od `bvapi_`. Otwiera tylko API: klucz MCP tu nie działa, a token nie działa dla MCP.

Każdy token ma kafelek z nazwą, informacją, czy może uruchamiać kopie, ostatnimi czterema znakami, tym, kiedy i skąd był ostatnio użyty, oraz dzisiejszymi wywołaniami. Na kafelku możesz zmienić jego nazwę i uprawnienia, wymienić go albo unieważnić. **Dziennik** pokazuje kopie, które uruchomił, i jego ostatnie wywołania. Przywrócenie konfiguracji BombVault z kopii unieważnia wszystkie tokeny, bo kopia może zawierać tokeny, które później unieważniłeś.

Bez hasła logowania każdy, kto może otworzyć interfejs WWW, może też utworzyć token. Jeśli otworzysz BombVault pod nazwą wyglądającą na publiczną i nie ma hasła, z tego adresu nie da się tworzyć tokenów, tak samo jak [kluczy MCP](mcp.md#switch-on).

## Punkty końcowe {#endpoints}

| Trasa | Co zwraca lub robi | Token |
|---|---|---|
| `GET /api/v1/health` | Wersja, nazwa instancji, czy trwa kopia i co może ten token | odczyt |
| `GET /api/v1/status` | Stan ochrony według obszaru: ostatnia udana kopia, oczekiwany odstęp, kontrole, najbliższe zaplanowane przebiegi | odczyt |
| `GET /api/v1/activity` | Co działa w tej chwili, z fazą i procentem | odczyt |
| `GET /api/v1/items` | Każdy chroniony element z harmonogramem, tym, co zatrzymuje kopia, i ostatnią kopią; `?domain=` dla jednego obszaru | odczyt |
| `GET /api/v1/runs` | Historia przebiegów, najnowsze najpierw; filtry `limit`, `domain`, `item`, `status`, `kind`, `since` | odczyt |
| `GET /api/v1/anomalies` | Anomalie z podsumowaniem otwartych; filtry `state`, `severity`, `domain`, `limit` | odczyt |
| `GET /api/v1/anomalies/{id}` | Jedna anomalia | odczyt |
| `GET /api/v1/storage/{domain}` | Historia rozmiaru, przyrost tygodniowy i wolne miejsce każdego repozytorium obszaru | odczyt |
| `POST /api/v1/backups` | Tworzy kopię jednego elementu (`{"domain":"containers","item":"plex"}`) albo całego obszaru (`{"domain":"vms"}`) | uruchamianie |
| `POST /api/v1/backups/everything` | Uruchamia Backup Everything | uruchamianie |
| `POST /api/v1/runs/{id}/cancel` | Przerywa trwającą kopię uruchomioną przez ten token | uruchamianie |

Obszary to `containers`, `vms`, `files`, `zfs`, `flash` i `config`. Czasy są w sekundach Unix. Odpowiedzi są takie same jak z [narzędzi MCP](mcp.md#tools) o tej samej nazwie, więc oba pozostają zgodne.

Kopia uruchomiona tutaj to ta sama kopia, którą uruchamia interfejs WWW: działający kontener jest zatrzymywany do końca swojej kopii. Żądanie wraca od razu, a `/api/v1/activity` i `/api/v1/runs` pokazują postęp.

## Przykłady {#examples}

```sh
# Jak mają się kopie?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Kopia jednego kontenera teraz.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

Z własnym samopodpisanym certyfikatem BombVault dodaj `--cacert bombvault-cert.pem` (plik z przycisku **Pobierz certyfikat** na karcie MCP) albo `-k` w zaufanej sieci.

## Błędy i limity {#errors}

Błąd wraca jako `{"error": {"code": "...", "message": "..."}}` z odpowiednim statusem:

| Status | Kody | Znaczenie |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Brakuje argumentu albo jest błędny |
| 401 | `no_token`, `invalid_token` | Brak tokenu albo token nieaktywny |
| 403 | `not_permitted` | Token może tylko czytać albo nie uruchomił tego przebiegu |
| 404 | `not_found` | Nie ma takiego elementu, przebiegu ani anomalii |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Coś innego już działa, obszar jest wyłączony albo nie ma nic do zrobienia |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Limit wstrzymuje żądanie; `Retry-After` mówi, kiedy spróbować ponownie |

Uruchomienia podlegają tym samym limitom co [uruchomienia przez MCP](mcp.md#starting-backups): 12 na godzinę na token, 15 minut między dwoma uruchomieniami tego samego elementu, najwyżej 4 uruchomienia elementu w ciągu 24 godzin oraz ochrona retencji. Ostatnie trzy liczą razem uruchomienia przez MCP, przez API i z Home Assistant. Token może wysłać 120 żądań na minutę. Pięć nieudanych prób z jednego adresu blokuje go na minutę.

## OpenAPI {#openapi}

BombVault udostępnia opis tych tras pod `/api/v1/openapi.json` (OpenAPI 3.1). Nie potrzeba do tego tokenu. Wczytaj go do Swagger UI, Postmana albo generatora kodu.

## Home Assistant {#home-assistant}

BombVault może pojawić się w Home Assistant jako urządzenie dzięki wykrywaniu MQTT. Home Assistant potrzebuje do tego swojej integracji MQTT i brokera, na przykład dodatku Mosquitto. Żaden własny komponent nie jest potrzebny.

1. W BombVault otwórz **Ustawienia, System, Home Assistant**.
2. Wpisz adres i port brokera, a także nazwę użytkownika i hasło, jeśli ich wymaga. Włącz **Użyj TLS**, jeśli broker używa TLS, zwykle na porcie 8883; jego certyfikat musi być ważny dla wpisanego adresu.
3. Włącz **Połącz z Home Assistant** i kliknij **Zapisz**. Karta pokazuje, kiedy połączenie działa.

Urządzenie nazywa się BombVault albo BombVault z nazwą instancji w nawiasie i ma takie encje:

| Encja | Co pokazuje |
|---|---|
| Status | `ok`, `warning`, `failed` lub `off`, najgorszy stan spośród włączonych obszarów |
| Running job | Co teraz działa, albo `idle` |
| Open anomalies | Ile anomalii jest otwartych |
| Next scheduled backup | Kiedy zaczyna się następna zaplanowana kopia |
| *Obszar* last backup | Kiedy wykonano ostatnią udaną kopię obszaru |
| *Obszar* last result | Jak zakończyła się jego ostatnia kopia |
| *Obszar* repository free space | Wolne miejsce tam, gdzie leży jego główne repozytorium, jeśli BombVault może je odczytać |
| Back up *obszar* | Przycisk, który kopiuje cały obszar |

Nazwy encji są po angielsku, bo Home Assistant przejmuje je tak, jak wysyła je BombVault. Każdy włączony obszar dostaje własne encje, a wyłączony je traci. Przyciski podlegają tym samym limitom co [uruchomienia przez API](#errors). Nacisnąć je może każdy, kto może publikować w brokerze, więc zabezpiecz broker hasłem albo wyłącz **Przyciski uruchamiają kopie**.

BombVault odczytuje swój stan co 15 sekund i publikuje go, gdy coś się zmieniło, jako JSON pod `<prefiks>/<węzeł>/state`. Prefiks to `bombvault`, dopóki go nie zmienisz, a węzeł to krótki identyfikator, który BombVault wybiera raz. Komunikaty wykrywania trafiają pod domyślny prefiks Home Assistant, `homeassistant`. Oba są zachowywane (retained). Ostatnia wola (last will) oznacza urządzenie jako niedostępne, jeśli BombVault zatrzyma się bez uprzedzenia. Wyłączenie połączenia usuwa urządzenie i jego encje z Home Assistant.

## Znajdowanie BombVault w sieci {#mdns}

BombVault ogłasza swój interfejs WWW w sieci lokalnej przez mDNS, protokół stojący za Bonjour i Avahi. Przeglądarka otwiera go wtedy jako `https://bombvault.local:3443`, a z `HTTP_ONLY` jako `http://bombvault.local:3000`, a przeglądarki usług pokazują go jako usługę WWW z podtypem `_bombvault`. Rekordy TXT zawierają wersję i ścieżkę. Przełącznik jest w **Ustawienia, System, Znajdź w sieci** i domyślnie jest włączony. Jeśli inne urządzenie używa już tej nazwy, BombVault bierze `bombvault-2.local` i tak dalej, a karta pokazuje otrzymany adres. Gdy BombVault się zatrzymuje albo wyłączysz ogłaszanie, informuje o tym sieć, a przeglądarki od razu usuwają wpis.

To, czy ogłoszenie dotrze do twojej sieci, zależy od tego, jak podłączony jest kontener:

- **bridge**, domyślne ustawienie w szablonie Unraid: ogłoszenie zostaje w sieci Dockera i nikt w sieci lokalnej go nie widzi. Otwieraj BombVault przez adres hosta jak dotąd.
- **br0** albo inna sieć macvlan lub ipvlan: kontener ma własny adres w sieci lokalnej i ogłoszenie tam dociera.
- **host**: ogłoszenie wychodzi przez interfejsy hosta, obok ogłoszenia samego Unraida.

Ogłaszane są tylko adresy IPv4.
