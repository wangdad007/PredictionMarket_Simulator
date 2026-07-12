# Prediction Market Simulator

这是一个独立的测试数据模拟器，用来模拟多个用户创建博弈池、给新池注入初始流动性、再由多个随机账户参与购买。

默认不会直接写数据库，也不会直接上链。它会先生成一份可检查的计划文件，你确认后再决定这批数据是只落库，还是发真实链上交易。

## 最快用法

只需要记住这条命令：

```bash
cd PredictionMarket_Simulator
go run .
```

在 `runtime.mode: "preview"` 下，程序会在终端里问你要生成哪类数据：

- 输入 `1`：创建新博弈池，并生成这些新池的购买数据。
- 输入 `2`：购买已有博弈池，随后输入已有的 `game_id`，例如 `1,2,3`。

这一步只生成预览计划，不写数据库、不上链。确认计划没问题后，再把 `runtime.mode` 改成 `"execute"` 执行同一个 `plan_file`。

## 两步流程

第一步：生成预览计划。

```bash
go run .
```

`go run .` 默认会读取 `config.yaml`，并在 `preview` 模式下从终端里询问要生成“创建新博弈池”还是“购买已有博弈池”的数据。

如果你想保留旧的非交互命令，仍然可以使用：

```bash
go run ./cmd/simulator -config config.yaml
```

旧命令不会弹出交互问题，会直接使用 `config.yaml` 里的 `scenario.type`。使用 `go run .` 时，程序会让你选择：

- `1` / `create_and_trade`：生成创建新博弈池的数据，并围绕这些新池生成购买记录。
- `2` / `trade_existing`：输入已有 `game_id`，只生成购买已有博弈池的数据。

交互输入只覆盖本次 `preview` 运行的配置，不会改写 `config.yaml`。`execute` 模式始终读取已经生成好的 `plan_file`，不会重新随机生成数据。

默认配置是：

```yaml
runtime:
  mode: "preview"
  plan_file: "out/simulator-plan.json"
```

运行后会打印本次随机生成的账户、博弈池、交易，并保存到 `out/simulator-plan.json`。

第二步：你看完计划后，再选择执行方式。

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

`approve_on_chain: true` 是额外确认开关。没有这个开关，即使 `on_chain: true`，模拟器也不会允许执行链上交易。

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

`trade_existing` 真正执行时需要 `runtime.on_chain: true`，因为已有池的储备、份额和价格应以链上状态为准；dry-run 模式可以只预览计划。

## 支持的博弈池类型

这些类型来自当前前端创建池代码：

- `TYPE_PRICE`
- `TYPE_VOLATILITY`
- `TYPE_VOLUME`
- `TYPE_TECHNICAL`
- `TYPE_TOUCH`
- `TYPE_RELATIVE`
- `TYPE_PRICE_THRESHOLD`
- `TYPE_EVENT`

模拟器会根据类型随机生成标题、条件、选项文案和 metadata。合约本身只保存 `ipfsCID` 和时间，类型信息属于池的 metadata/数据库展示层。

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

允许随机创建的池类型。留空会使用全部 8 种类型。

`market.initial_liquidity_min/max_bkc`

每个新池初始流动性的随机范围，单位是 BKC。

`market.duration_min/max_seconds`

每个新池持续时间的随机范围，单位是秒。

`trade.buy_min/max_bkc`

每笔购买金额的随机范围，单位是 BKC。

`trade.creator_also_trades`

创建者是否也可以参与购买自己创建的池。

`timing.pause_seconds`

每笔交易之间暂停多久，避免请求打得太密。

`timing.timeout_seconds`

整次模拟运行的超时时间。

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
