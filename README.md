# Prediction Market Simulator

这是一个独立的测试数据模拟器，可让多个随机账户在不同时间购买 YES/NO。它支持两种数据模式：为已有博弈池继续生成交易，或创建新博弈池后生成交易。

## 最快用法

只需要记住这条命令：

```bash
cd PredictionMarket_Simulator
go run .
```

程序会直接询问：

- 输入 `1`：为已有博弈池生成购买数据，然后输入 `game_id`，例如 `1,2,3`。
- 输入 `2`：创建新博弈池并生成购买数据，然后输入每轮创建数量。

交互选择只影响本次运行，不会改写 `config.yaml`。在 execute 模式下，选择后会自动生成对应的新计划，避免误执行旧计划。

## 参数选择

已有博弈池模式：

```bash
go run . -scenario existing -game-ids 1,2,3
```

创建新博弈池模式：

```bash
go run . -scenario create -market-count 3
```

参数模式不会再弹出问题，适合后台运行。也可以只传 `-game-ids` 或 `-market-count`，模拟器会自动判断模式。

## 数据写入方式

只写数据库，不上链：

```yaml
runtime:
  mode: "execute"
  on_chain: false
  plan_file: "out/simulator-plan.json"
mysql:
  dsn: "..."
```

真实上链并同步数据库：

```yaml
runtime:
  mode: "execute"
  on_chain: true
  approve_on_chain: true
  plan_file: "out/simulator-plan.json"
chain:
  private_key: "..."
mysql:
  dsn: "..."
```

`approve_on_chain: true` 是额外确认开关。没有这个开关，即使 `on_chain: true`，模拟器也不会允许执行链上交易。`runtime.mode: preview` 可用于只生成并检查计划，不写数据库、不发送链上交易。

## 执行模式

`runtime.mode: preview`

只生成并展示计划，不写数据库，不发链上交易。计划文件里保存了随机账户、博弈池和交易金额，所以后续执行的是你已经看过的同一批数据。

注意：计划文件包含模拟账户私钥，只用于本地测试，不要提交或分享。`out/` 已经被 `.gitignore` 忽略。

`runtime.mode: execute` + `runtime.on_chain: false`

只写数据库，不发链上交易。模拟器会读取你已经预览过的计划，按计划创建博弈池、购买 YES/NO，并把结果写入：

- `gold_games`
- `gold_chain_states`
- `gold_user_positions`
- `gold_trades`
- `gold_price_history`
- `market_history`

这适合快速给前端和后端接口准备展示数据。注意：这种模式下链上合约并不知道这些 `game_id`。

`runtime.mode: execute` + `runtime.on_chain: true`

先发真实链上交易，再同步数据库。流程是：

1. 用 `chain.private_key` 对应的钱包给随机生成的模拟账户打测试币。
2. 随机账户调用 `createGame(ipfsCID, duration)` 创建博弈池。
3. 随机账户调用 `buyShares(gameId, optionId)` 购买 YES/NO。
4. 读取链上 `getGameInfo` / `getGameExtraData`，把真实链上状态同步到数据库。

## 真实用户行为

默认 `scenario.type: create_and_trade`，不是只买现有池。它会模拟：

- 随机账户 A 创建博弈池
- 随机账户 B/C/D... 买入该池
- 多个池循环执行
- 创建者是否也参与购买由 `trade.creator_also_trades` 控制

如果只想压测已有池，可以改成：

```yaml
scenario:
  type: "trade_existing"
  existing_game_ids: [1, 2, 3]
```

`trade_existing` 在 `on_chain: false` 时从 MySQL 的 `gold_games`、`gold_chain_states` 恢复池状态并继续模拟；在 `on_chain: true` 时读取链上状态并发送真实购买交易。已开奖、退款或已截止的池不会继续生成购买数据。

## 支持的博弈池类型

新建市场只使用可由 Chainlink XAU/USD 和 BTC/USD 数据轮次复算的 6 种类型：

- `TYPE_PRICE`
- `TYPE_RETURN_THRESHOLD`
- `TYPE_PRICE_THRESHOLD`
- `TYPE_PRICE_RANGE`
- `TYPE_RELATIVE`
- `TYPE_STREAK`

模拟器会生成 `rule_version: 2` 元数据，起止时间固定为北京时间有效零点。`create_and_trade` 创建未来整日市场并逐步添加交易；`trade_existing` 只对现有 `game_id` 生成交易，不修改原市场规则。

## 主要配置

`runtime.enabled`

是否允许运行模拟器。它是安全开关，不是前端展示开关。设为 `false` 且处于 `execute` 模式时会直接停止，防止误执行；`preview` 模式仍可生成计划文件。

`runtime.mode`

`preview` 表示只生成计划文件；`execute` 表示读取计划文件并执行。

`runtime.on_chain`

执行阶段是否发真实链上交易。`false` 是 DB-only 模拟，`true` 是真实调用合约。

`runtime.plan_file`

预览阶段写入、执行阶段读取的计划文件；是否在执行前覆盖它由 `regenerate_plan_on_execute` 控制。

`runtime.regenerate_plan_on_execute`

控制 execute 模式是否在每次启动时重新生成随机计划。设置为 `true` 时，`go run .` 会生成新的账户、市场模板顺序、参数、持续时间、流动性和交易，并覆盖 `plan_file` 后立即执行；设置为 `false` 时会重复执行已有计划，适合复现问题。真实上链时开启该选项会在每次运行创建新市场，请谨慎使用。

`runtime.continuous`

设为 `true` 后，模拟器会常驻运行。每轮重新生成模拟账户、博弈池和交易计划，在每笔购买前等待随机时间；一轮完成后再等待随机轮次间隔并开始下一轮。此选项只支持 `mode: execute`，并要求 `regenerate_plan_on_execute: true`。设为 `false` 时仍保持原来的一次执行后退出。

`runtime.approve_on_chain`

上链执行的确认开关。只有 `mode: execute`、`on_chain: true`、`approve_on_chain: true` 同时满足时，才会发链上交易。

`runtime.dry_run`

旧版预览开关。现在推荐使用 `runtime.mode: preview`。

`chain.private_key`

上链执行模式需要。这个账户会先给随机模拟账户转测试币，随机账户再各自创建和购买。

`chain.contract_address`

预测市场合约地址。

`chain.rpc_url`

本地或普通 EVM RPC 地址，`use_broker_chain: false` 时使用。

`chain.broker_chain_url` / `chain.use_broker_chain`

是否改用 BrokerChain 接口发交易和读链。开启后仍会使用 `private_key` 给随机账户签名。

`ipfs.upload_url`

在 `runtime.on_chain: true` 创建博弈池前，模拟器会把 metadata 上传到此地址，并将返回的 CID 写入链和数据库。默认 `http://127.0.0.1:8081/api/v1/ipfs/add` 使用后端本地内容服务，适合本机联调；如已运行 Kubo，可改为 `http://127.0.0.1:5001/api/v0/add?pin=true`。

`mysql.dsn`

数据库连接串。`mode: execute` 时必须填写。

`scenario.market_count`

本次随机创建多少个博弈池。

`scenario.participants`

随机模拟账户数量。

`scenario.trades_per_market_min/max`

每个博弈池会随机产生多少笔购买交易。

`market.types`

允许随机创建的池类型。留空会使用全部 6 种可确定性裁决类型。

`market.initial_liquidity_min/max_bkc`

每个新池初始流动性的随机范围，单位是 BKC。

`market.duration_min/max_seconds`

每个新池观察期的随机范围，单位是秒。只接受 1-4 个整天，即 `86400` 到 `345600`。

`trade.buy_min/max_bkc`

每笔购买金额的随机范围，单位是 BKC。

`trade.creator_also_trades`

创建者是否也可以参与购买自己创建的池。

`timing.pause_seconds`

单次交易完成后的短暂停顿，用于降低数据库、链和网关压力。

`timing.trade_interval_min_seconds` / `timing.trade_interval_max_seconds`

控制同一博弈池中相邻两笔购买之间的随机等待时间。新生成的计划会把每笔等待秒数写入 `delay_seconds`，因此可以从计划文件和运行日志核对购买节奏。

`timing.cycle_interval_min_seconds` / `timing.cycle_interval_max_seconds`

只在常驻模式使用，控制一轮完成后到下一轮开始前的随机等待时间。

`timing.timeout_seconds`

单轮模拟的超时时间。常驻模式不会因为这个值结束整个后台进程；某一轮失败后会记录日志，并在轮次间隔后继续尝试。

## 常驻后台运行

先确认 `config.yaml` 中已设置：

```yaml
runtime:
  enabled: true
  mode: "execute"
  continuous: true
  regenerate_plan_on_execute: true
```

建议先构建，再放到后台运行：

```bash
mkdir -p out
go build -o out/predictionmarket-simulator .
nohup ./out/predictionmarket-simulator -interactive=false > out/simulator.log 2>&1 &
echo $! > out/simulator.pid
```

也可以在后台命令中直接指定模式。例如持续交易已有池：

```bash
nohup ./out/predictionmarket-simulator -scenario existing -game-ids 1,2,3 > out/simulator.log 2>&1 &
```

持续创建新池：

```bash
nohup ./out/predictionmarket-simulator -scenario create -market-count 3 > out/simulator.log 2>&1 &
```

查看实时日志：

```bash
tail -f out/simulator.log
```

平滑停止：

```bash
kill "$(cat out/simulator.pid)"
```

## 前端是否会显示

`runtime.enabled: true` 只代表允许模拟器执行，不代表生成的数据一定会在前端出现。

- `runtime.mode: preview`：只生成 `plan_file`，不写数据库、不上链，前端不会显示新数据。
- `runtime.mode: execute` + `runtime.on_chain: false`：会把计划写入 MySQL。如果前端后端连接的就是同一个数据库，并且页面读取这些表，通常可以显示；但链上合约并不知道这些 DB-only 数据。
- `runtime.mode: execute` + `runtime.on_chain: true`：会发真实链上交易并同步数据库。前端是否显示取决于交易成功、同步成功，以及前端后端是否读取同一套链和数据库。

## YES/NO 编号约定

现有前端、后端和合约都使用：

- `option_id = 0` 表示 YES
- `option_id = 1` 表示 NO

但链上 `getGameExtraData` 返回的储备数组顺序是 `[reserveNO, reserveYES]`。模拟器内部已经按这个顺序换算价格，DB-only 模式也按合约的恒定乘积公式模拟买入后的储备和份额变化。
