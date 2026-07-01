# Tables
Tables are the primary general-purpose data structure of Luna. They function as arrays, hashmaps, and objects. They are high-performance, easy to use, and incredibly flexible.

Tables may be assigned to variables using *table definition syntax*. An empty table is always defined with `[]`. Tables may also be defined with elements already in them, using `key => value` or just `value1, value2`.

## Lists
When a table has exclusively incrementing integer keys which begin at 0, the table is known as a `list`. A table may be queried for if it is a list with `table.isList()`. Internally, such tables are stored as contiguous blocks of memory, e.g. arrays. As soon as a key of another type is used, the table is internally converted to a hashmap implementation. If elements are instead removed, or added under new integer keys, the table may still internally be represented contiguous in memory. That may be queried for using `table.isContiguousMemory()`. The memory overhead of `list`s as compared to raw arrays in other languages is minimal.
