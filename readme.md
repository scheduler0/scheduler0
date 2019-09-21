# Cron Server

Fault tolerant cron job server.

## Dependencies

    1: Postgres
    2: Redis
    3: Rabbitmq

## Installation
    
    1: Build container using host network  `docker build . --tag="cron-server:latest" --network="host"`
    2: Run container `docker run -d -p 8080:8080 cron-server`
    
# License

This project is licensed under either of

 * Apache License, Version 2.0, ([LICENSE-APACHE](LICENSE-APACHE) or
   http://www.apache.org/licenses/LICENSE-2.0)
 * MIT license ([LICENSE-MIT](LICENSE-MIT) or
   http://opensource.org/licenses/MIT)

at your option.

### Contribution

Unless you explicitly state otherwise, any contribution intentionally submitted
for inclusion in this project by you, as defined in the Apache-2.0 license,
shall be dual licensed as above, without any additional terms or conditions.