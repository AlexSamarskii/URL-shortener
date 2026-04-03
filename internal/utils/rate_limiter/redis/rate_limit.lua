local key = KEYS[1]
local now = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local capacity = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])

local tokens = redis.call('HGET', key, 'tokens')
local last_refill = redis.call('HGET', key, 'last_refill')

if tokens == false then
    tokens = capacity
    last_refill = now
else
    tokens = tonumber(tokens)
    last_refill = tonumber(last_refill)
    local delta = math.max(0, (now - last_refill) * rate)
    tokens = math.min(capacity, tokens + delta)
end

last_refill = now

if tokens >= cost then
    tokens = tokens - cost
    redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
    redis.call('EXPIRE', key, math.ceil(capacity / rate) + 1)
    return 1
else
    redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
    redis.call('EXPIRE', key, math.ceil(capacity / rate) + 1)
    return 0
end