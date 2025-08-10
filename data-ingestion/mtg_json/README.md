
https://mtgjson.com/getting-started/

duckdb


SELECT * FROM read_parquet('myCostReport/MyCostReport/**/**/*.parquet');

SELECT * FROM read_parquet('data-ingestion/**/data/cards.parquet');


Check the schema
DESCRIBE  SELECT * FROM read_parquet('data-ingestion/**/data/cards.parquet');



SELECT subtypes, sum(1) FROM read_parquet('data-ingestion/**/data/cards.parquet') where subtypes like '%Angel%' group by subtypes;




SELECT subtypes, sum(1) FROM read_parquet('data-ingestion/**/data/cards.parquet') where subtypes like '%Angel%' group by subtypes;


SELECT * FROM read_parquet('data-ingestion/**/data/cards.parquet');
 

SELECT name, subtypes, text FROM read_parquet('data-ingestion/**/data/cards.parquet') where text like '%draw each turn%';
