// ============================================================================
// 祠堂渲染层 —— 真 3D 实现（Three.js）
// 木牌有真实厚度、文字用 CanvasTexture 贴图（随 3D 透视正确变形）、
// 方向光 + 阴影 + 环境光，形成明暗面和体积感。
// 与 HallRenderer2D5 同契约：mount/render/onPick/highlight/setWatermark/capture/dispose
// ============================================================================
import * as THREE from 'three'
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js'
import { getWatermarkLevel } from './watermark.js'

// 中文自动换行：按字符测量，超出 maxWidth 折行
function wrapText(ctx, text, maxWidth) {
  const chars = Array.from(text)
  const lines = []
  let cur = ''
  for (const ch of chars) {
    if (cur && ctx.measureText(cur + ch).width > maxWidth) {
      lines.push(cur)
      cur = ch
    } else {
      cur += ch
    }
  }
  if (cur) lines.push(cur)
  return lines.length ? lines : [text]
}

export class HallRenderer3D {
  constructor() {
    this.container = null
    this.scene = null
    this.camera = null
    this.renderer = null
    this._pickCb = null
    this._raycaster = new THREE.Raycaster()
    this._tabletGroup = new THREE.Group()
    this._stepGroup = new THREE.Group()
    this._houseGroup = new THREE.Group()
    this._houseFadeMats = []
    this._plaqueMeshes = []     // 祖训大匾（可点击）
    this._houseInfo = null
    this._watermarkEl = null
    this._raf = null
    this._highlightId = null
    this._panel = null
    this._downPos = null
    // 视角状态：'first' 第一人称 / 'free' 自由环绕
    this._viewMode = 'first'
    this._camPos = new THREE.Vector3(0, 2.4, 8)   // 第一人称眼睛位置
    this._camYaw = Math.PI
    this._camPitch = 0.12
    this._eyeH = 2.4
    this._freeTarget = null    // 自由视角环绕中心
    this._freeCamPos = null    // 自由视角默认相机位置
    this._controls = null
    this._fov = 42             // 第一人称视野角（滚轮缩放范围 20~90）
    this._freeYaw = 0          // 自由视角（旁观飞行）朝向
    this._freePitch = 0
    this._flySpeed = 0.3       // 旁观飞行速度（固定）
    this._pointerLocked = false  // 旁观模式右键切换指针锁定状态
    this._firstLayout = true
    this._keys = {}
    this._dragging = false
    this._lastPointer = null
    this._pinchDist = null      // 双指捏合缩放初始距离
    this._walkMinZ = 2
    this._walkMaxZ = 26
    this._walkMinX = -26
    this._walkMaxX = 26
    this._moveInput = { fwd: 0, strafe: 0 }
    this._joystick = null
    this._joyTouchId = null
    this._joyOrigin = null
    // 香 + 烟雾粒子
    this._incenses = []
    this._incenseBase = null
    this._incenseBurnTime = 1800  // 一炷香时长（秒）= 古代 30 分钟
    this._smokePoints = null
    this._smokeGeo = null
    this._smokeMat = null
    this._incenseCounter = null
    this._incenseTotalCount = 0
    this._lastT = 0
  }

  mount(container) {
    this.container = container
    const w = container.clientWidth || 800
    const h = container.clientHeight || 600

    // 场景 + 深色背景 + 雾（景深）
    this.scene = new THREE.Scene()
    this.scene.background = new THREE.Color('#171008')
    this.scene.fog = new THREE.Fog('#171008', 25, 70)

    // 透视相机（第一人称=眼睛 / 自由=环绕，可切换）
    this.camera = new THREE.PerspectiveCamera(42, w / h, 0.1, 200)
    this.camera.position.set(0, 2, 12)
    this.camera.lookAt(0, 1.4, 0)

    // 渲染器
    this.renderer = new THREE.WebGLRenderer({ antialias: true })
    this.renderer.setSize(w, h)
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2))
    this.renderer.shadowMap.enabled = true
    this.renderer.shadowMap.type = THREE.PCFSoftShadowMap
    if ('outputColorSpace' in this.renderer) this.renderer.outputColorSpace = THREE.SRGBColorSpace
    container.appendChild(this.renderer.domElement)
    // 移动端：去掉点击高亮，避免点击牌位时闪一下
    this.renderer.domElement.style.webkitTapHighlightColor = 'transparent'
    this.renderer.domElement.style.touchAction = 'none'

    // 自由环绕控制器（初始禁用，切到「自由视角」时启用）
    this._controls = new OrbitControls(this.camera, this.renderer.domElement)
    this._controls.enabled = false
    this._controls.enableDamping = true
    this._controls.dampingFactor = 0.08
    this._controls.maxPolarAngle = Math.PI * 0.52
    this._controls.minPolarAngle = Math.PI * 0.05
    this._controls.minDistance = 6
    this._controls.maxDistance = 80
    this._controls.zoomToCursor = true

    this._setupLights()
    this._setupGround()

    this.scene.add(this._stepGroup)
    this.scene.add(this._tabletGroup)
    this.scene.add(this._houseGroup)

    // 水印 overlay（CSS 层，盖在 canvas 上）
    this._watermarkEl = document.createElement('div')
    Object.assign(this._watermarkEl.style, {
      position: 'absolute', inset: '0', pointerEvents: 'none', zIndex: '10', display: 'none',
    })
    container.appendChild(this._watermarkEl)

    // 准星（旁观模式锁定时显示，Minecraft 式）
    this._crosshairEl = document.createElement('div')
    this._crosshairEl.style.cssText = 'position:absolute;left:50%;top:50%;width:24px;height:24px;transform:translate(-50%,-50%);pointer-events:none;z-index:30;display:none;'
    this._crosshairEl.innerHTML =
      '<div style="position:absolute;left:50%;top:50%;width:2px;height:20px;background:#fff;transform:translate(-50%,-50%);box-shadow:0 0 4px rgba(0,0,0,.8);"></div>' +
      '<div style="position:absolute;left:50%;top:50%;width:20px;height:2px;background:#fff;transform:translate(-50%,-50%);box-shadow:0 0 4px rgba(0,0,0,.8);"></div>'
    container.appendChild(this._crosshairEl)

    this.renderer.domElement.addEventListener('wheel', this._onWheel, { passive: false })
    this.renderer.domElement.addEventListener('mousedown', this._onMouseDown)
    this.renderer.domElement.addEventListener('click', this._onClick)
    this.renderer.domElement.addEventListener('mousemove', this._onMove)
    this.renderer.domElement.addEventListener('mouseup', this._onMouseUp)
    this.renderer.domElement.addEventListener('mouseleave', this._onMouseUp)
    this.renderer.domElement.addEventListener('contextmenu', this._onContextMenu)
    this.renderer.domElement.addEventListener('touchstart', this._onTouchStart, { passive: false })
    this.renderer.domElement.addEventListener('touchmove', this._onTouchMove, { passive: false })
    this.renderer.domElement.addEventListener('touchend', this._onTouchEnd)
    window.addEventListener('keydown', this._onKeyDown)
    window.addEventListener('keyup', this._onKeyUp)
    window.addEventListener('resize', this._onResize)
    document.addEventListener('pointerlockchange', this._onPointerLockChange)
    this._buildJoystick()

    this._animate()
  }

  _setupLights() {
    // 环境光（柔，照亮暗部）
    this.scene.add(new THREE.AmbientLight('#7a6a50', 1.1))
    // 半球光（天空暖色 + 地面暗色，环境反射）
    this.scene.add(new THREE.HemisphereLight('#d8c8a0', '#241708', 0.7))
    // 方向光（主光，明暗 + 阴影）
    const dir = new THREE.DirectionalLight('#fff1d0', 2.6)
    dir.position.set(10, 18, 12)
    dir.castShadow = true
    dir.shadow.mapSize.set(2048, 2048)
    dir.shadow.camera.left = -40
    dir.shadow.camera.right = 40
    dir.shadow.camera.top = 40
    dir.shadow.camera.bottom = -40
    dir.shadow.camera.near = 1
    dir.shadow.camera.far = 90
    dir.shadow.bias = -0.0003
    this.scene.add(dir)
  }

  _setupGround() {
    // 地面（供桌底座），顶面在 y=0，台阶堆叠落地
    const geo = new THREE.BoxGeometry(80, 0.4, 60)
    const mat = new THREE.MeshStandardMaterial({ color: '#241708', roughness: 0.85 })
    const ground = new THREE.Mesh(geo, mat)
    ground.position.y = -0.2
    ground.receiveShadow = true
    this.scene.add(ground)
  }

  // 相机更新：第一人称=眼睛位置；自由视角=旁观飞行（都看向朝向方向）
  _updateCamera() {
    if (this._viewMode === 'free') {
      const p = this._freeCamPos || this.camera.position
      const cp = Math.cos(this._freePitch)
      const dirX = Math.sin(this._freeYaw) * cp
      const dirY = Math.sin(this._freePitch)
      const dirZ = Math.cos(this._freeYaw) * cp
      this.camera.position.copy(p)
      this.camera.lookAt(p.x + dirX, p.y + dirY, p.z + dirZ)
      return
    }
    if (this._viewMode !== 'first') return
    const cp = Math.cos(this._camPitch)
    const dirX = Math.sin(this._camYaw) * cp
    const dirY = Math.sin(this._camPitch)
    const dirZ = Math.cos(this._camYaw) * cp
    this.camera.position.copy(this._camPos)
    this.camera.lookAt(
      this._camPos.x + dirX,
      this._camPos.y + dirY,
      this._camPos.z + dirZ
    )
  }

  // 移动（键盘 + 摇杆）：第一人称有边界；自由视角=旁观飞行（无碰撞、可上下）
  _updateMove() {
    const isFree = this._viewMode === 'free'
    if (!isFree && this._viewMode !== 'first') return
    const yaw = isFree ? this._freeYaw : this._camYaw
    const fwd = { x: Math.sin(yaw), z: Math.cos(yaw) }
    let mx = 0, mz = 0, my = 0
    if (this._keys['w'] || this._keys['arrowup']) { mx += fwd.x; mz += fwd.z }
    if (this._keys['s'] || this._keys['arrowdown']) { mx -= fwd.x; mz -= fwd.z }
    if (this._keys['d'] || this._keys['arrowright']) { mx -= fwd.z; mz += fwd.x }
    if (this._keys['a'] || this._keys['arrowleft']) { mx += fwd.z; mz -= fwd.x }
    if (this._moveInput.fwd) { mx += fwd.x * this._moveInput.fwd; mz += fwd.z * this._moveInput.fwd }
    if (this._moveInput.strafe) { mx -= fwd.z * this._moveInput.strafe; mz += fwd.x * this._moveInput.strafe }
    if (isFree) {
      // 旁观飞行：空格上升、Shift 下降
      if (this._keys[' ']) my += 1
      if (this._keys['shift']) my -= 1
    }

    const len = Math.hypot(mx, mz, my)
    if (len > 0.05) {
      const speed = isFree ? this._flySpeed : 0.12
      let nx = mx, nz = mz, ny = my
      if (len > 1) { nx /= len; nz /= len; ny /= len }
      if (isFree) {
        this._freeCamPos.x += nx * speed
        this._freeCamPos.y += ny * speed
        this._freeCamPos.z += nz * speed
      } else {
        this._camPos.x += nx * speed
        this._camPos.z += nz * speed
        this._camPos.x = Math.max(this._walkMinX, Math.min(this._walkMaxX, this._camPos.x))
        this._camPos.z = Math.max(this._walkMinZ, Math.min(this._walkMaxZ, this._camPos.z))
      }
    }
  }

  // 滚轮：第一人称 + 自由视角（旁观）都=缩放视野角（放大缩小）
  _onWheel = (e) => {
    if (this._viewMode !== 'first' && this._viewMode !== 'free') return
    e.preventDefault()
    this._fov += (e.deltaY > 0 ? 1 : -1) * 2.5
    this._fov = Math.max(20, Math.min(90, this._fov))
    if (this.camera) {
      this.camera.fov = this._fov
      this.camera.updateProjectionMatrix()
    }
  }

  // 切换视角：'first' 第一人称 / 'free' 旁观飞行（可自由移动穿墙）
  setViewMode(mode) {
    if (mode === this._viewMode) return
    this._viewMode = mode
    this._applyHouseOpacity()
    if (this._controls) this._controls.enabled = false
    if (mode === 'first') {
      if (document.exitPointerLock && document.pointerLockElement === this.renderer.domElement) {
        document.exitPointerLock()
      }
      this._updateCamera()
    } else {
      if (this._freeCamPos) this.camera.position.copy(this._freeCamPos)
      this._updateCamera()
    }
  }

  // 移动端虚拟摇杆（左摇杆移动）
  _buildJoystick() {
    const joy = document.createElement('div')
    Object.assign(joy.style, {
      position: 'absolute', left: '24px', bottom: '24px',
      width: '96px', height: '96px', borderRadius: '50%',
      background: 'rgba(255,255,255,0.12)',
      border: '1px solid rgba(255,255,255,0.3)',
      zIndex: '5', display: 'none'
    })
    const knob = document.createElement('div')
    Object.assign(knob.style, {
      position: 'absolute', left: '34px', top: '34px',
      width: '28px', height: '28px', borderRadius: '50%',
      background: 'rgba(255,255,255,0.55)', pointerEvents: 'none'
    })
    joy.appendChild(knob)
    this.container.appendChild(joy)
    this._joystick = { el: joy, knob }

    // 仅在触摸设备显示
    if ('ontouchstart' in window) joy.style.display = 'block'

    joy.addEventListener('touchstart', (e) => {
      e.preventDefault()
      this._joyTouchId = e.changedTouches[0].identifier
      const r = joy.getBoundingClientRect()
      this._joyOrigin = { x: r.left + r.width / 2, y: r.top + r.height / 2 }
      this._updateJoystick(e.changedTouches[0])
    }, { passive: false })
    joy.addEventListener('touchmove', (e) => {
      for (const t of e.changedTouches) {
        if (t.identifier === this._joyTouchId) {
          e.preventDefault()
          this._updateJoystick(t)
        }
      }
    }, { passive: false })
    const reset = () => {
      this._joyTouchId = null
      this._moveInput = { fwd: 0, strafe: 0 }
      knob.style.left = '34px'
      knob.style.top = '34px'
    }
    joy.addEventListener('touchend', (e) => {
      for (const t of e.changedTouches) if (t.identifier === this._joyTouchId) reset()
    })
    joy.addEventListener('touchcancel', reset)
  }

  _updateJoystick(t) {
    if (!this._joyOrigin) return
    const dx = t.clientX - this._joyOrigin.x
    const dy = t.clientY - this._joyOrigin.y
    const max = 34
    const len = Math.hypot(dx, dy)
    const cl = Math.min(len, max)
    const nx = len > 0 ? dx / len : 0
    const ny = len > 0 ? dy / len : 0
    this._moveInput = { fwd: -ny * (cl / max), strafe: nx * (cl / max) }
    this._joystick.knob.style.left = (34 + nx * cl) + 'px'
    this._joystick.knob.style.top = (34 + ny * cl) + 'px'
  }

  render(layout, info) {
    this._clearHall()
    this._houseInfo = info || null
    this._buildHall(layout)
  }

  _clearHall() {
    const disposeObj = (o) => {
      if (o.geometry) o.geometry.dispose()
      if (o.material) {
        const mats = Array.isArray(o.material) ? o.material : [o.material]
        mats.forEach((m) => {
          if (m.map) m.map.dispose()
          m.dispose()
        })
      }
    }
    while (this._stepGroup.children.length) {
      this._stepGroup.children.pop().traverse(disposeObj)
    }
    while (this._tabletGroup.children.length) {
      this._tabletGroup.children.pop().traverse(disposeObj)
    }
    while (this._houseGroup.children.length) {
      this._houseGroup.children.pop().traverse(disposeObj)
    }
    this._houseFadeMats = []
    this._plaqueMeshes = []
    this._incenses = []
    this._incenseCounter = null
    this._incenseTotalCount = 0
    if (this._smokePoints) {
      this.scene.remove(this._smokePoints)
      if (this._smokeGeo) this._smokeGeo.dispose()
      if (this._smokeMat) this._smokeMat.dispose()
      this._smokePoints = null
      this._smokeGeo = null
      this._smokeMat = null
    }
  }

  _buildHall(layout) {
    const rows = layout.rows || []
    const N = rows.length
    // 没有牌位也渲染基础场景（供桌 + 香炉 + 图腾 + 房子），只是不摆牌位

    const stepH = 0.35       // 每级台阶高度差（台阶高度 = (N-ri)*stepH，第一世最高）
    const stepDepth = 1.1    // 台阶前后深度（减半）
    const tabletW = 0.46     // 牌位宽
    const tabletH = 1.1      // 牌位高
    const tabletD = 0.14     // 牌位厚
    const gap = 0.3          // 牌位间距
    const zStep = stepDepth * 0.72  // 每级台阶往前的距离

    // 统一宽度（所有台阶同宽 = 最宽一排，规整像供桌）；无牌位时用默认宽度
    const maxN = N ? Math.max(...rows.map((r) => r.members.length)) : 5
    const tableW = maxN * (tabletW + gap) + 1.2

    const baseH = 2.0          // 贡品台高度
    const frontDepth = 3.4     // 前方空位总深度（香炉 + 贡品）

    const nSteps = Math.max(N, 1)                   // 至少一层（无牌位时保留供桌基准）
    const stepsZ = (nSteps - 1) * zStep + stepDepth  // 台阶堆叠总深度
    const baseDepth = stepsZ + frontDepth            // 底座总深度
    const backZ = -(nSteps - 1) * zStep - stepDepth / 2 // 底座后缘
    const baseCenterZ = backZ + baseDepth / 2

    // 材质（明暗面）
    const baseMat = new THREE.MeshStandardMaterial({ color: '#4a3018', roughness: 0.6 })
    const baseTopMat = new THREE.MeshStandardMaterial({ color: '#7a5532', roughness: 0.55 })
    const stepTopMat = new THREE.MeshStandardMaterial({ color: '#9a7040', roughness: 0.55 })
    const stepFaceMat = new THREE.MeshStandardMaterial({ color: '#3a2a18', roughness: 0.7 })
    const tabTopMat = new THREE.MeshStandardMaterial({ color: '#a67c4e', roughness: 0.5 })
    const tabSideMat = new THREE.MeshStandardMaterial({ color: '#3a2818', roughness: 0.65 })

    // 底座（供桌主体，整体落地，台阶堆叠架在其上）
    const baseGeo = new THREE.BoxGeometry(tableW + 1.6, baseH, baseDepth)
    const baseMats = [baseMat, baseMat, baseTopMat, baseMat, baseMat, baseMat]
    const base = new THREE.Mesh(baseGeo, baseMats)
    base.position.set(0, baseH / 2, baseCenterZ)
    base.receiveShadow = true
    base.castShadow = true
    this._stepGroup.add(base)

    // 台阶（每个独立，底面都放在贡品台面 y=baseH，第一世最高、逐级降低）
    rows.forEach((row, ri) => {
      const n = row.members.length
      const stepH_actual = (N - ri) * stepH  // 本台阶高度（第一世最高）
      const stepTopY = baseH + stepH_actual  // 台阶顶面
      const zCenter = -(N - 1 - ri) * zStep

      const stepGeo = new THREE.BoxGeometry(tableW, stepH_actual, stepDepth)
      const stepMats = [stepFaceMat, stepFaceMat, stepTopMat, stepFaceMat, stepFaceMat, stepFaceMat]
      const step = new THREE.Mesh(stepGeo, stepMats)
      step.position.set(0, baseH + stepH_actual / 2, zCenter)  // 底面 y=baseH
      step.receiveShadow = true
      step.castShadow = true
      this._stepGroup.add(step)

      const startX = -((n - 1) * (tabletW + gap)) / 2
      row.members.forEach((m, mi) => {
        const tablet = this._buildTablet(m, tabTopMat, tabSideMat, tabletW, tabletH, tabletD)
        tablet.position.set(startX + mi * (tabletW + gap), stepTopY, zCenter)
        this._tabletGroup.add(tablet)
      })
    })

    // 香炉（中间）+ 贡品果盘（两侧）+ 蜡烛（贡品外侧）
    this._buildIncense(0, baseH, stepDepth / 2 + 1.0)
    this._buildOffering(-1.3, baseH, stepDepth / 2 + 1.7)
    this._buildOffering(1.3, baseH, stepDepth / 2 + 1.7)
    this._buildCandle(-2.2, baseH, stepDepth / 2 + 1.0)
    this._buildCandle(2.2, baseH, stepDepth / 2 + 1.0)

    // 第一人称：眼睛站在祠堂厅堂中间，面向牌位；划定可行走范围
    const altarFrontZ = baseCenterZ + baseDepth / 2
    this._walkMinZ = altarFrontZ + 2.5
    this._walkMaxZ = this._walkMinZ + 18
    this._walkMinX = -tableW / 2 - 8
    this._walkMaxX = tableW / 2 + 8
    if (this._firstLayout) {
      this._camPos.set(0, this._eyeH, this._walkMinZ + 6)
      this._camYaw = Math.PI
      this._camPitch = 0.12
      this._firstLayout = false
    }
    this._camPos.x = Math.max(this._walkMinX, Math.min(this._walkMaxX, this._camPos.x))
    this._camPos.z = Math.max(this._walkMinZ, Math.min(this._walkMaxZ, this._camPos.z))

    // 自由环绕视角：中心在供桌，默认俯视 15°
    const totalH = baseH + N * stepH
    const center = new THREE.Vector3(0, totalH / 2, baseCenterZ)
    const dz = baseDepth + 8
    const dx = 8
    const horiz = Math.sqrt(dx * dx + dz * dz)
    const dy = Math.tan(15 * Math.PI / 180) * horiz
    this._freeTarget = center
    this._freeCamPos = new THREE.Vector3(center.x + dx, center.y + dy, center.z + dz)
    if (this._controls) this._controls.target.copy(center)
    // 初始化旁观飞行朝向：看向祠堂中心
    const _d = new THREE.Vector3().subVectors(center, this._freeCamPos)
    if (_d.lengthSq() > 1e-4) {
      _d.normalize()
      this._freeYaw = Math.atan2(_d.x, _d.z)
      this._freePitch = Math.asin(Math.max(-1, Math.min(1, _d.y)))
    }

    this._buildHouse(tableW, backZ, baseCenterZ, totalH, this._walkMinZ)

    this._updateCamera()
  }

  // 祠堂建筑（半开放室内空间）：木柱 + 房梁 + 后墙 + 格栅侧墙 + 屋顶 + 灯笼
  _buildHouse(tableW, backZ, baseCenterZ, totalH, walkMinZ) {
    const HS = 2                        // 房子放大系数（用户 21:37 要求加大一倍）
    const HW = tableW / 2 + 5 * HS      // 半宽（左右墙位置）
    const backZ_w = backZ - 3 * HS      // 后墙 z
    const frontZ_w = walkMinZ + 8 * HS  // 前墙 z（第一人称站立区后方）
    const totemSize = 2.4              // 图腾牌匾正方形边长（缩小后 = 原高度）
    const roofY = totalH + 12  // 高度：原 8 → 加大一倍 16 → 降增加的一半 → 12
    const beamLen = frontZ_w - backZ_w // 房子纵深

    // 材质
    const pillarMat = new THREE.MeshStandardMaterial({ color: '#3a2818', roughness: 0.7 })
    const beamMat = new THREE.MeshStandardMaterial({ color: '#4a3018', roughness: 0.7 })
    const wallMat = new THREE.MeshStandardMaterial({ color: '#2a1c10', roughness: 0.75 })
    const roofMat = new THREE.MeshStandardMaterial({ color: '#241708', roughness: 0.8 })
    const paperMat = new THREE.MeshStandardMaterial({
      color: '#e8a020', emissive: '#ff8c00', emissiveIntensity: 1.4, roughness: 0.5,
    })

    // 需随视角切换透明的材质（屋顶 + 墙 + 柱子 + 横梁 + 吊灯）
    this._houseFadeMats = [wallMat, roofMat, pillarMat, beamMat, paperMat]

    const g = this._houseGroup

    // 四角木柱
    const pillarGeo = new THREE.CylinderGeometry(0.35, 0.4, roofY, 12)
    const addPillar = (x, z) => {
      const p = new THREE.Mesh(pillarGeo, pillarMat)
      p.position.set(x, roofY / 2, z)
      p.castShadow = true
      p.receiveShadow = true
      g.add(p)
    }
    addPillar(-HW, frontZ_w); addPillar(HW, frontZ_w)
    addPillar(-HW, backZ_w); addPillar(HW, backZ_w)

    // 顶部房梁（矩形框架）
    const beamD = 0.5
    const beamGeoX = new THREE.BoxGeometry(HW * 2, beamD, beamD)
    const beamGeoZ = new THREE.BoxGeometry(beamD, beamD, beamLen)
    const beamY = roofY - beamD / 2
    const addBeam = (geo, x, y, z) => {
      const b = new THREE.Mesh(geo, beamMat)
      b.position.set(x, y, z)
      b.castShadow = true
      g.add(b)
    }
    addBeam(beamGeoX, 0, beamY, frontZ_w)
    addBeam(beamGeoX, 0, beamY, backZ_w)
    addBeam(beamGeoZ, -HW, beamY, (frontZ_w + backZ_w) / 2)
    addBeam(beamGeoZ, HW, beamY, (frontZ_w + backZ_w) / 2)

    // 后墙（深色木墙）
    const wallGeo = new THREE.BoxGeometry(HW * 2, roofY, 0.4)
    const wall = new THREE.Mesh(wallGeo, wallMat)
    wall.position.set(0, roofY / 2, backZ_w)
    wall.receiveShadow = true
    g.add(wall)

    // 两侧实心墙（取消窗户/格栅）
    const buildSide = (x) => {
      const side = new THREE.Mesh(new THREE.BoxGeometry(0.4, roofY, beamLen), wallMat)
      side.position.set(x, roofY / 2, (frontZ_w + backZ_w) / 2)
      side.receiveShadow = true
      g.add(side)
    }
    buildSide(-HW)
    buildSide(HW)

    // 前墙（实心，中间留大门洞）
    const doorW = 4.4
    const doorH = 4.8
    const segW = (HW * 2 - doorW) / 2
    const addFrontSeg = (x, w, y, h) => {
      const seg = new THREE.Mesh(new THREE.BoxGeometry(w, h, 0.4), wallMat)
      seg.position.set(x, y, frontZ_w)
      seg.receiveShadow = true
      g.add(seg)
    }
    addFrontSeg(-(doorW / 2 + segW / 2), segW, roofY / 2, roofY)           // 门洞左侧墙
    addFrontSeg(doorW / 2 + segW / 2, segW, roofY / 2, roofY)               // 门洞右侧墙
    addFrontSeg(0, doorW, doorH + (roofY - doorH) / 2, roofY - doorH)       // 门楣（门洞上方）

    // 屋顶（平板，自由视角时半透明）
    const roof = new THREE.Mesh(new THREE.BoxGeometry(HW * 2 + 2, 0.4, beamLen + 2), roofMat)
    roof.position.set(0, roofY + 0.2, (frontZ_w + backZ_w) / 2)
    roof.receiveShadow = true
    g.add(roof)

    // 灯笼（房梁下垂，暖黄发光）
    const lanternX = HW * 0.5
    const ropeGeo = new THREE.CylinderGeometry(0.03, 0.03, 1.4, 6)
    const bodyGeo = new THREE.CylinderGeometry(0.5, 0.62, 0.95, 16)
    const addLantern = (x, z) => {
      const rope = new THREE.Mesh(ropeGeo, beamMat)
      rope.position.set(x, roofY - 0.7, z)
      g.add(rope)
      const body = new THREE.Mesh(bodyGeo, paperMat)
      body.position.set(x, roofY - 2.2, z)
      body.castShadow = true
      g.add(body)
    }
    addLantern(-lanternX, baseCenterZ)
    addLantern(lanternX, baseCenterZ)

    // 图腾牌匾（中间堂号/姓氏 + 两边祖训 + 底图 + 样式）
    const info = this._houseInfo || {}
    const surname = info.surname || ''
    const hallName = info.hallName || ''
    const totemImage = info.totemImage || ''
    const totemStyle = info.totemStyle || 'wood'
    const mottoes = info.mottoes || []
    const totemFontSize = info.totemFontSize || 200
    if (hallName || surname || mottoes.length || totemImage) {
      const totemMat = new THREE.MeshStandardMaterial({ roughness: 0.6 })
      // 图腾不参与房子透明化：自由视角时墙/屋顶透明，图腾保持不透明（否则看不清堂号/底图）
      // 正方形牌匾，底部抬高到原顶部位置（totalH + 2.6）
      const totem = new THREE.Mesh(new THREE.BoxGeometry(totemSize, totemSize, 0.25), totemMat)
      totem.position.set(0, totalH + 2.6 + totemSize / 2, backZ_w + 0.4)
      g.add(totem)
      // 异步生成纹理（可能需要加载底图）
      this._buildTotemTexture(totemMat, { surname, hallName, totemImage, totemStyle, mottoes, totemFontSize })
    }

    // 祖训大匾（图腾左右两侧，竖排显示祖训）
    if (mottoes.length) {
      const half = Math.ceil(mottoes.length / 2)
      const leftList = mottoes.slice(0, half)
      const rightList = mottoes.slice(half)
      const plaqueW = 0.9
      const plaqueH = 3.6
      const plaqueGap = 0.7
      const plaqueCenterX = totemSize / 2 + plaqueGap + plaqueW / 2
      const plaqueY = totalH + 2.6 + plaqueH / 2   // 底部对齐图腾底部 totalH + 2.6
      const addPlaque = (list, x) => {
        if (!list.length) return
        const mat = new THREE.MeshStandardMaterial({ roughness: 0.6 })
        mat.map = this._makeMottoTexture(list, totemStyle)
        mat.needsUpdate = true
        const mesh = new THREE.Mesh(new THREE.BoxGeometry(plaqueW, plaqueH, 0.22), mat)
        mesh.position.set(x, plaqueY, backZ_w + 0.4)
        mesh.userData.kind = 'motto'
        mesh.userData.mottoes = list
        mesh.userData.style = totemStyle
        g.add(mesh)
        this._plaqueMeshes.push(mesh)
      }
      addPlaque(leftList, -plaqueCenterX)
      addPlaque(rightList, plaqueCenterX)
    }

    // 凡例牌匾（房子右墙上，竖长匾，每条凡例一个，沿墙从里到外排）
    const legends = info.legends || []
    if (legends.length) {
      const lgW = 0.9
      const lgH = 3.6
      const lgGap = 0.7
      const lgY = totalH + 2.6 + lgH / 2
      const lgX = HW - 0.35                    // 右墙内侧
      const startZ = backZ_w + 2.0             // 从后墙往前排
      legends.forEach((lg, i) => {
        const z = startZ + i * (lgW + lgGap)
        const mat = new THREE.MeshStandardMaterial({ roughness: 0.6 })
        mat.map = this._makeMottoTexture([lg], totemStyle)
        mat.needsUpdate = true
        // 厚度沿 x（贴墙），宽沿 z，高沿 y；±x 面为匾正面（面向房间内）
        const mesh = new THREE.Mesh(new THREE.BoxGeometry(0.22, lgH, lgW), mat)
        mesh.position.set(lgX, lgY, z)
        mesh.userData.kind = 'legend'
        mesh.userData.legends = [lg]
        mesh.userData.style = totemStyle
        g.add(mesh)
        this._plaqueMeshes.push(mesh)
      })
    }

    this._applyHouseOpacity()
  }

  // 异步生成图腾牌匾纹理：底色(样式) → 底图 → 边框 → 文字(堂号/姓氏 + 祖训)
  async _buildTotemTexture(totemMat, opts) {
    const { surname, hallName, totemImage, totemStyle, totemFontSize } = opts
    const c = document.createElement('canvas')
    const W = 1024, H = 1024
    c.width = W
    c.height = H
    const ctx = c.getContext('2d')

    // 样式配色
    const st = this._getTotemStyle(totemStyle)

    // 1. 底色
    const grad = ctx.createLinearGradient(0, 0, W, 0)
    grad.addColorStop(0, st.bg[0])
    grad.addColorStop(0.5, st.bg[1])
    grad.addColorStop(1, st.bg[2])
    ctx.fillStyle = grad
    ctx.fillRect(0, 0, W, H)

    // 2. 底图（半透明底纹，居中等比缩放）
    if (totemImage) {
      try {
        const img = await this._loadImage(totemImage)
        const pad = 60
        const s = Math.min((W - pad * 2) / img.width, (H - pad * 2) / img.height)
        const dw = img.width * s
        const dh = img.height * s
        ctx.globalAlpha = 0.45
        ctx.drawImage(img, (W - dw) / 2, (H - dh) / 2, dw, dh)
        ctx.globalAlpha = 1
      } catch (e) {
        console.warn('[图腾] 底图加载失败，跳过：', e)
      }
    }

    // 3. 边框
    ctx.strokeStyle = st.border
    ctx.lineWidth = 10
    ctx.strokeRect(12, 12, W - 24, H - 24)

    // 4. 中间堂号/姓氏大字
    const centerText = hallName || surname || ''
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.fillStyle = st.text
    ctx.font = `bold ${totemFontSize || 200}px "Songti SC", "SimSun", serif`
    ctx.fillText(centerText, W / 2, H / 2)

    const tex = new THREE.CanvasTexture(c)
    tex.colorSpace = THREE.SRGBColorSpace
    tex.anisotropy = 4
    totemMat.map = tex
    totemMat.needsUpdate = true
  }

  // 牌匾样式配色表
  _getTotemStyle(style) {
    const STYLES = {
      wood:  { bg: ['#3a2412', '#4a3018', '#33200f'], border: '#c8a96a', text: '#f3d98c', sub: '#e8d8b0' },
      red:   { bg: ['#6a1410', '#8a1c16', '#5a100c'], border: '#f0c860', text: '#fbe9b8', sub: '#f5d9a0' },
      blue:  { bg: ['#12294a', '#1a3860', '#0e1e38'], border: '#c8a96a', text: '#e8dcc0', sub: '#d8c8a8' },
      stone: { bg: ['#4a4a48', '#5a5a58', '#3e3e3c'], border: '#b8b8b0', text: '#e8e4dc', sub: '#d0ccc4' },
    }
    return STYLES[style] || STYLES.wood
  }

  // 祖训大匾纹理（竖长匾，竖排显示祖训）
  _makeMottoTexture(mottoes, style) {
    const c = document.createElement('canvas')
    const W = 512, H = 2048
    c.width = W
    c.height = H
    const ctx = c.getContext('2d')
    const st = this._getTotemStyle(style)

    // 底色
    const grad = ctx.createLinearGradient(0, 0, W, 0)
    grad.addColorStop(0, st.bg[0])
    grad.addColorStop(0.5, st.bg[1])
    grad.addColorStop(1, st.bg[2])
    ctx.fillStyle = grad
    ctx.fillRect(0, 0, W, H)

    // 边框
    ctx.strokeStyle = st.border
    ctx.lineWidth = 10
    ctx.strokeRect(14, 14, W - 28, H - 28)

    // 竖排祖训（每条一列，水平居中）——取 title（无 title 用 content）
    const list = (mottoes || [])
      .map((x) => (typeof x === 'string' ? x : (x && (x.title || x.content)) || ''))
      .filter((t) => t && t.trim())
    if (list.length) {
      ctx.textAlign = 'center'
      ctx.textBaseline = 'middle'
      ctx.fillStyle = st.text
      const maxChars = Math.max(...list.map((t) => Array.from(t).length))
      const lh = Math.min(160, (H - 300) / Math.max(maxChars, 1))
      const fs = Math.floor(lh * 0.85)
      const colW = fs + 16
      ctx.font = `bold ${fs}px "Songti SC", "SimSun", serif`
      const totalW = list.length * colW
      let cx = W / 2 - totalW / 2 + colW / 2
      list.forEach((t) => {
        const chars = Array.from(t)
        const totalH = chars.length * lh
        let sy = (H - totalH) / 2 + lh / 2
        chars.forEach((ch, i) => ctx.fillText(ch, cx, sy + i * lh))
        cx += colW
      })
    }

    const tex = new THREE.CanvasTexture(c)
    tex.colorSpace = THREE.SRGBColorSpace
    tex.anisotropy = 4
    return tex
  }

  // 加载图片（Promise 包装，跨域匿名）
  _loadImage(url) {
    return new Promise((resolve, reject) => {
      const img = new Image()
      img.crossOrigin = 'anonymous'
      img.onload = () => resolve(img)
      img.onerror = () => reject(new Error('图片加载失败'))
      img.src = url
    })
  }

  // 视角切换时更新房子材质透明度：第一人称不透明 / 自由视角透明（避免挡牌位）
  _applyHouseOpacity() {
    const free = this._viewMode === 'free'
    this._houseFadeMats.forEach((m) => {
      if (!m) return
      m.transparent = free
      m.opacity = free ? 0.15 : 1
      m.needsUpdate = true
    })
  }

  _buildIncense(x, y, z) {
    // 青铜香炉：炉身 + 炉口 + 动态香（燃烧变短）+ 烟雾粒子
    const bronze = new THREE.MeshStandardMaterial({ color: '#7a5a2e', roughness: 0.4, metalness: 0.5 })
    const rimMat = new THREE.MeshStandardMaterial({ color: '#4a3218', roughness: 0.5, metalness: 0.3 })

    const body = new THREE.Mesh(new THREE.CylinderGeometry(0.42, 0.5, 0.6, 24), bronze)
    body.position.set(x, y + 0.3, z)
    body.castShadow = true
    body.receiveShadow = true
    this._stepGroup.add(body)

    const rim = new THREE.Mesh(new THREE.CylinderGeometry(0.47, 0.47, 0.12, 24), rimMat)
    rim.position.set(x, y + 0.64, z)
    this._stepGroup.add(rim)

    // 香插点（香底部从炉口顶部伸出，避免穿模）
    this._incenseBase = { x, y: y + 0.68, z }
    this._incenses = []
    // 基础 3 支：固定错落位置（有前有后、有左有右），每次渲染不变
    const basePositions = [
      { x: 0, z: -0.12 },
      { x: -0.16, z: 0.1 },
      { x: 0.16, z: 0.1 },
    ]
    basePositions.forEach((p) => {
      this._incenses.push(this._makeIncense(p.x, this._incenseBase.y, p.z, false))
    })

    // 上香记录（服务端同步）：每条 offer 创建 count 支细香，预设燃烧进度
    // 显示上限 60 支（含固定 3 支），超出不渲染；后台不限制上香总数
    const DISPLAY_MAX = 60
    const BONUS_MAX = DISPLAY_MAX - 3
    const info = this._houseInfo || {}
    const offers = info.offers || []
    // 先独立计算实际总香数（不受渲染截断影响）
    let totalIncense = 3
    for (const o of offers) {
      totalIncense += (o.count || 3)
    }
    // 再渲染（受 60 上限截断）
    let bonusRendered = 0
    for (const o of offers) {
      const n = o.count || 3
      for (let i = 0; i < n; i++) {
        if (bonusRendered >= BONUS_MAX) break
        const p = this._randomIncensePos()
        this._incenses.push(this._makeIncense(p.x, this._incenseBase.y, p.z, true, o.burn || 0))
        bonusRendered++
      }
      if (bonusRendered >= BONUS_MAX) break
    }

    // 香炉计数牌：显示当前香总数量（实际总数，不受显示上限影响）
    this._buildIncenseCounter(x, y, z, totalIncense)

    // 香炉烟雾粒子
    this._initSmoke()
  }

  // 香炉计数牌：在香炉中间用 Sprite 显示当前香总数量（实时更新）
  _buildIncenseCounter(x, y, z, total) {
    const canvas = document.createElement('canvas')
    canvas.width = 256
    canvas.height = 112
    const ctx = canvas.getContext('2d')
    const texture = new THREE.CanvasTexture(canvas)
    texture.minFilter = THREE.LinearFilter
    const sprite = new THREE.Sprite(new THREE.SpriteMaterial({ map: texture, transparent: true, depthTest: false }))
    sprite.scale.set(0.6, 0.26, 1)
    sprite.position.set(x, y + 0.3, z)
    sprite.renderOrder = 999
    this._stepGroup.add(sprite)
    this._incenseCounter = { sprite, canvas, ctx, texture }
    this._incenseTotalCount = total
    this._refreshIncenseCounter()
  }

  // 重画计数牌（实时更新数字）
  _refreshIncenseCounter() {
    const c = this._incenseCounter
    if (!c) return
    const { canvas, ctx, texture } = c
    ctx.clearRect(0, 0, canvas.width, canvas.height)
    ctx.beginPath()
    ctx.roundRect(6, 6, 244, 100, 18)
    ctx.fillStyle = 'rgba(28, 18, 10, 0.78)'
    ctx.fill()
    ctx.strokeStyle = '#c9a86a'
    ctx.lineWidth = 3
    ctx.stroke()
    ctx.fillStyle = '#f5e6c8'
    ctx.font = 'bold 48px "PingFang SC", "Microsoft YaHei", sans-serif'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.fillText(`${this._incenseTotalCount} 支`, 128, 62)
    texture.needsUpdate = true
  }

  // 公开：外部同步计数牌总数（多用户轮询用，只更新数字不重渲染香）
  setIncenseCount(total) {
    if (total == null) return
    this._incenseTotalCount = total
    this._refreshIncenseCounter()
  }

  // 单根香（可燃烧变短）：香杆 + 香头红点，offsetX/offsetZ 为相对香炉中心的偏移
  // 上香香（isBonus）细一点：香杆半径 0.013，基础香 0.02
  _makeIncense(offsetX, baseY, offsetZ, isBonus, burn = 0) {
    const g = new THREE.Group()
    const stickMat = new THREE.MeshStandardMaterial({ color: '#8a5a30', roughness: 0.7 })
    const tipMat = new THREE.MeshStandardMaterial({ color: '#c0392b', roughness: 0.6, emissive: '#5a1008', emissiveIntensity: 0.7 })

    const r = isBonus ? 0.013 : 0.02
    const stickGeo = new THREE.CylinderGeometry(r, r + 0.005, 0.9, 8)
    stickGeo.translate(0, 0.45, 0)  // 底部移到 y=0
    const stick = new THREE.Mesh(stickGeo, stickMat)
    stick.castShadow = true
    g.add(stick)

    const tipR = isBonus ? 0.025 : 0.035
    const tip = new THREE.Mesh(new THREE.SphereGeometry(tipR, 8, 8), tipMat)
    tip.position.y = 0.9
    g.add(tip)

    const wx = (this._incenseBase ? this._incenseBase.x : 0) + offsetX
    const wz = (this._incenseBase ? this._incenseBase.z : 0) + offsetZ
    g.position.set(wx, baseY, wz)
    this._stepGroup.add(g)

    const inc = { group: g, stick, tip, burn, isBonus: !!isBonus, baseX: wx, baseY, baseZ: wz }
    this._applyIncenseBurn(inc)
    return inc
  }

  // 上香：增加 3 支香（烧完消失），基础 3 支永久循环
  addIncense() {
    if (!this._incenseBase) return
    const b = this._incenseBase
    for (let i = 0; i < 3; i++) {
      const p = this._randomIncensePos()
      this._incenses.push(this._makeIncense(p.x, b.y, p.z, true))
    }
    this._incenseTotalCount = (this._incenseTotalCount || 0) + 3
    this._refreshIncenseCounter()
  }

  // 香炉口径内随机位置（限制在炉内，不超出炉壁）
  _randomIncensePos() {
    const r = 0.28 * Math.sqrt(Math.random())
    const ang = Math.random() * Math.PI * 2
    return { x: Math.cos(ang) * r, z: Math.sin(ang) * r }
  }

  // 应用燃烧进度：香杆变短 + 香头下降 + 香头渐暗
  _applyIncenseBurn(inc) {
    const h = Math.max(1 - inc.burn, 0.03)
    inc.stick.scale.y = h
    inc.tip.position.y = 0.9 * h
    inc.tip.material.emissiveIntensity = 0.7 * Math.max(0, 1 - inc.burn)
  }

  // 每帧更新：香燃烧变短；基础香烧完重燃，上香香烧完移除
  _updateIncense(dt) {
    if (!this._incenses || !this._incenses.length) return
    const burnTime = this._incenseBurnTime || 1800
    const toRemove = []
    for (const inc of this._incenses) {
      inc.burn += dt / burnTime
      if (inc.burn >= 1) {
        if (inc.isBonus) toRemove.push(inc)
        else inc.burn = 0
      }
      this._applyIncenseBurn(inc)
    }
    for (const inc of toRemove) {
      this._incenses = this._incenses.filter((x) => x !== inc)
      this._stepGroup.remove(inc.group)
      inc.group.traverse((o) => {
        if (o.geometry) o.geometry.dispose()
        if (o.material) {
          const ms = Array.isArray(o.material) ? o.material : [o.material]
          ms.forEach((m) => m.dispose())
        }
      })
    }
    if (toRemove.length) {
      this._incenseTotalCount = Math.max(3, this._incenseTotalCount - toRemove.length)
      this._refreshIncenseCounter()
    }
  }

  // 香头世界位置（烟雾发射点）
  _incenseTips() {
    const tips = []
    for (const inc of this._incenses || []) {
      if (inc.burn >= 1) continue
      tips.push({
        x: inc.baseX,
        y: inc.baseY + 0.9 * Math.max(1 - inc.burn, 0.03),
        z: inc.baseZ,
      })
    }
    return tips
  }

  // 烟雾粒子系统
  _initSmoke() {
    const MAX = 48
    this._smokeMax = MAX
    this._smokePos = new Float32Array(MAX * 3)
    this._smokeVel = []
    this._smokeAge = new Float32Array(MAX)
    this._smokeLife = new Float32Array(MAX)
    for (let i = 0; i < MAX; i++) {
      this._smokeAge[i] = 999
      this._smokeLife[i] = 2 + Math.random() * 2
      this._smokeVel.push(new THREE.Vector3())
    }
    this._smokeGeo = new THREE.BufferGeometry()
    this._smokeGeo.setAttribute('position', new THREE.BufferAttribute(this._smokePos, 3))
    this._smokeMat = new THREE.PointsMaterial({
      color: '#c8beb2',
      size: 0.16,
      transparent: true,
      opacity: 0.45,
      depthWrite: false,
    })
    this._smokePoints = new THREE.Points(this._smokeGeo, this._smokeMat)
    this._smokePoints.frustumCulled = false
    this.scene.add(this._smokePoints)
  }

  // 每帧更新烟雾：从香头飘出，上升扩散
  _updateSmoke(dt) {
    if (!this._smokePoints) return
    const emitters = this._incenseTips()
    for (let i = 0; i < this._smokeMax; i++) {
      this._smokeAge[i] += dt
      if (this._smokeAge[i] >= this._smokeLife[i]) {
        this._smokeAge[i] = 0
        this._smokeLife[i] = 2.5 + Math.random() * 2
        this._smokeVel[i].set(
          (Math.random() - 0.5) * 0.2,
          0.4 + Math.random() * 0.3,
          (Math.random() - 0.5) * 0.2
        )
        if (emitters.length) {
          const e = emitters[(Math.random() * emitters.length) | 0]
          this._smokePos[i * 3] = e.x + (Math.random() - 0.5) * 0.05
          this._smokePos[i * 3 + 1] = e.y
          this._smokePos[i * 3 + 2] = e.z + (Math.random() - 0.5) * 0.05
        } else {
          this._smokePos[i * 3 + 1] = -999
        }
      } else {
        this._smokePos[i * 3] += this._smokeVel[i].x * dt
        this._smokePos[i * 3 + 1] += this._smokeVel[i].y * dt
        this._smokePos[i * 3 + 2] += this._smokeVel[i].z * dt
      }
    }
    this._smokeGeo.attributes.position.needsUpdate = true
  }

  _buildOffering(x, y, z) {
    // 贡品果盘：盘子 + 三个水果（红苹果/橙橘子/黄梨）
    const plateMat = new THREE.MeshStandardMaterial({ color: '#b8a07a', roughness: 0.5 })
    const fruitMats = [
      new THREE.MeshStandardMaterial({ color: '#c0392b', roughness: 0.35 }),
      new THREE.MeshStandardMaterial({ color: '#e67e22', roughness: 0.4 }),
      new THREE.MeshStandardMaterial({ color: '#f1c40f', roughness: 0.4 }),
    ]

    const plate = new THREE.Mesh(new THREE.CylinderGeometry(0.36, 0.28, 0.08, 24), plateMat)
    plate.position.set(x, y + 0.04, z)
    plate.castShadow = true
    plate.receiveShadow = true
    this._stepGroup.add(plate)

    const fruitGeo = new THREE.SphereGeometry(0.1, 16, 16)
    const fruitPos = [
      [0, 0.18, 0],
      [0.16, 0.14, 0.05],
      [-0.16, 0.14, -0.04],
    ]
    fruitPos.forEach((p, i) => {
      const fruit = new THREE.Mesh(fruitGeo, fruitMats[i])
      fruit.position.set(x + p[0], y + p[1], z + p[2])
      fruit.castShadow = true
      this._stepGroup.add(fruit)
    })
  }

  _buildCandle(x, y, z) {
    // 蜡烛：红色烛身 + 发光火焰
    const waxMat = new THREE.MeshStandardMaterial({ color: '#c0392b', roughness: 0.4 })
    const flameMat = new THREE.MeshStandardMaterial({ color: '#f7b731', roughness: 0.3, emissive: '#f39c12', emissiveIntensity: 1.6 })

    const wax = new THREE.Mesh(new THREE.CylinderGeometry(0.07, 0.08, 0.6, 16), waxMat)
    wax.position.set(x, y + 0.3, z)
    wax.castShadow = true
    this._stepGroup.add(wax)

    const flame = new THREE.Mesh(new THREE.ConeGeometry(0.05, 0.18, 12), flameMat)
    flame.position.set(x, y + 0.7, z)
    this._stepGroup.add(flame)
  }

  _buildTablet(m, topMat, sideMat, w, h, d) {
    const group = new THREE.Group()

    // 主体（木牌，带文字）
    const geo = new THREE.BoxGeometry(w, h, d)
    const frontTex = this._makeTabletTexture(m)
    const frontMat = new THREE.MeshStandardMaterial({ map: frontTex, roughness: 0.55 })
    // 材质顺序：[+x,-x,+y,-y,+z,-z] 右、左、上、下、前、后
    const mats = [sideMat, sideMat, topMat, sideMat, frontMat, sideMat]
    const tablet = new THREE.Mesh(geo, mats)
    tablet.userData.memberId = m.id
    tablet.castShadow = true
    tablet.position.y = h / 2 + h * 0.06
    group.add(tablet)

    // 底座（稍宽的矮台）
    const baseH = h * 0.12
    const baseGeo = new THREE.BoxGeometry(w * 1.5, baseH, d * 1.5)
    const base = new THREE.Mesh(baseGeo, topMat)
    base.userData.memberId = m.id
    base.castShadow = true
    base.receiveShadow = true
    base.position.y = baseH / 2
    group.add(base)

    group.userData.memberId = m.id
    return group
  }

  _makeTabletTexture(m) {
    const c = document.createElement('canvas')
    const W = 128, H = 256
    c.width = W
    c.height = H
    const ctx = c.getContext('2d')

    // 木纹底色
    const grad = ctx.createLinearGradient(0, 0, W, 0)
    grad.addColorStop(0, '#4a3520')
    grad.addColorStop(0.5, '#543d24')
    grad.addColorStop(1, '#3f2d18')
    ctx.fillStyle = grad
    ctx.fillRect(0, 0, W, H)
    // 细木纹
    ctx.strokeStyle = 'rgba(0,0,0,0.12)'
    for (let i = 0; i < 8; i++) {
      const y = 20 + i * 28
      ctx.beginPath()
      ctx.moveTo(0, y)
      ctx.lineTo(W, y + 6)
      ctx.stroke()
    }
    // 边框
    ctx.strokeStyle = '#c8a96a'
    ctx.lineWidth = 5
    ctx.strokeRect(7, 7, W - 14, H - 14)

    // 名字（竖排，居中）
    const name = (m.name || '').trim()
    const chars = name.split('')
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.fillStyle = '#f3e5c8'
    ctx.font = 'bold 46px "Songti SC", "SimSun", serif'
    const lineH = 52
    const totalH = chars.length * lineH
    let startY = (H - totalH) / 2 + lineH / 2
    // 名字太长时缩小
    if (totalH > H - 40) {
      ctx.font = 'bold 36px "Songti SC", "SimSun", serif'
      const lh2 = 40
      startY = (H - chars.length * lh2) / 2 + lh2 / 2
      chars.forEach((ch, i) => ctx.fillText(ch, W / 2, startY + i * lh2))
    } else {
      chars.forEach((ch, i) => ctx.fillText(ch, W / 2, startY + i * lineH))
    }

    // 字辈（右下小字竖排）
    if (m.generation_word) {
      const gz = m.generation_word.split('')
      ctx.font = '24px "Songti SC", "SimSun", serif'
      ctx.fillStyle = '#d8c8a0'
      const lh = 26
      const sy = H - 26 - gz.length * lh
      gz.forEach((ch, i) => ctx.fillText(ch, W - 22, sy + i * lh))
    }

    const tex = new THREE.CanvasTexture(c)
    tex.colorSpace = THREE.SRGBColorSpace
    tex.anisotropy = 4
    return tex
  }

  _onMouseDown = (e) => {
    if (e.button === 2) {
      // 右键：切换锁定/解锁（旁观 + 第一人称通用）
      if (this._viewMode === 'free' || this._viewMode === 'first') {
        if (this._pointerLocked) {
          if (document.exitPointerLock) document.exitPointerLock()
        } else {
          const el = this.renderer.domElement
          const p = el.requestPointerLock && el.requestPointerLock()
          if (p && p.catch) p.catch(() => {})
        }
      }
      return
    }
    if (e.button === 0) {
      this._downPos = { x: e.clientX, y: e.clientY }
      if (this._viewMode === 'free') {
        if (!this._pointerLocked) {
          // 未锁定：左键拖拽转视角
          this._dragging = true
          this._lastPointer = { x: e.clientX, y: e.clientY }
        }
      } else if (this._viewMode === 'first') {
        if (!this._pointerLocked) {
          // 未锁定：左键拖拽转身
          this._dragging = true
          this._lastPointer = { x: e.clientX, y: e.clientY }
        }
      }
    }
  }

  _onMouseUp = () => {
    this._dragging = false
  }

  _onContextMenu = (e) => {
    // 旁观/第一人称阻止右键菜单弹出（右键用于切换锁定）
    if (this._viewMode === 'free' || this._viewMode === 'first') e.preventDefault()
  }

  _onPointerLockChange = () => {
    this._pointerLocked = document.pointerLockElement === this.renderer.domElement
    if (this._crosshairEl) {
      this._crosshairEl.style.display = this._pointerLocked ? 'block' : 'none'
    }
  }

  _onKeyDown = (e) => {
    const k = e.key.toLowerCase()
    // 阻止游戏键默认行为：空格/方向键会触发聚焦按钮（如“切换视角”按钮）或滚动页面
    const t = e.target
    const isInput = t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)
    if (!isInput && (k === ' ' || k === 'shift' || k.startsWith('arrow'))) {
      e.preventDefault()
    }
    this._keys[k] = true
  }

  _onKeyUp = (e) => {
    this._keys[e.key.toLowerCase()] = false
  }

  _onTouchStart = (e) => {
    if (e.touches.length === 1) {
      this._downPos = { x: e.touches[0].clientX, y: e.touches[0].clientY }
      if (this._viewMode === 'first' || this._viewMode === 'free') {
        this._dragging = true
        this._lastPointer = { x: e.touches[0].clientX, y: e.touches[0].clientY }
      }
    } else if (e.touches.length === 2) {
      // 双指捏合：记录初始距离，取消拖拽
      this._dragging = false
      this._pinchDist = Math.hypot(
        e.touches[0].clientX - e.touches[1].clientX,
        e.touches[0].clientY - e.touches[1].clientY
      )
    }
  }

  _onTouchMove = (e) => {
    if ((this._viewMode === 'first' || this._viewMode === 'free') && e.touches.length === 2 && this._pinchDist != null) {
      // 双指捏合缩放视野角：张开放大（拉近）、捏拢拉远
      const d = Math.hypot(
        e.touches[0].clientX - e.touches[1].clientX,
        e.touches[0].clientY - e.touches[1].clientY
      )
      const delta = (d - this._pinchDist) * 0.06
      this._fov = Math.max(20, Math.min(90, this._fov - delta))
      if (this.camera) {
        this.camera.fov = this._fov
        this.camera.updateProjectionMatrix()
      }
      this._pinchDist = d
      if (e.cancelable) e.preventDefault()
    } else if ((this._viewMode === 'first' || this._viewMode === 'free') && e.touches.length === 1 && this._dragging) {
      const t = e.touches[0]
      const dx = t.clientX - this._lastPointer.x
      const dy = t.clientY - this._lastPointer.y
      this._lastPointer = { x: t.clientX, y: t.clientY }
      if (this._viewMode === 'free') {
        this._freeYaw -= dx * 0.006
        this._freePitch = Math.max(-1.4, Math.min(1.4, this._freePitch - dy * 0.006))
      } else {
        this._camYaw -= dx * 0.006
        this._camPitch = Math.max(-1.2, Math.min(1.2, this._camPitch - dy * 0.006))
      }
      this._downPos = { x: t.clientX, y: t.clientY }
      if (e.cancelable) e.preventDefault()
    }
  }

  _onTouchEnd = () => {
    this._dragging = false
    this._pinchDist = null
  }

  _onClick = (e) => {
    if (!this._pickCb) return
    // 锁定模式（旁观/第一人称 + 指针锁定）：从屏幕中心（准星）拾取
    if (this._pointerLocked && (this._viewMode === 'free' || this._viewMode === 'first')) {
      // 弹窗已打开：左键关闭弹窗
      if (this._panel) {
        this.hideMemberPanel()
        return
      }
      const hit = this._pickAtCenter()
      if (!hit) return
      const wp = new THREE.Vector3()
      hit.getWorldPosition(wp)
      this._handleHit(hit, wp)
      return
    }
    // 拖动旋转后松开：若鼠标移动过，视为旋转而非点击（防误触）
    if (this._downPos) {
      const dx = e.clientX - this._downPos.x
      const dy = e.clientY - this._downPos.y
      this._downPos = null
      if (dx * dx + dy * dy > 36) return
    }
    // 点击已显示的面板 → 关闭
    if (this._panel && this._pickPanel(e)) {
      this.hideMemberPanel()
      return
    }
    const hit = this._pickAt(e)
    if (!hit) return
    const wp = new THREE.Vector3()
    hit.getWorldPosition(wp)
    this._handleHit(hit, wp)
  }

  // 命中牌匾/牌位后的统一处理
  _handleHit(hit, wp) {
    // 祖训大匾：直接在牌匾前面弹出祖训内容
    if (hit.userData.kind === 'motto') {
      this.showMottoPanel(hit.userData.mottoes, hit.userData.style, wp)
      return
    }
    // 凡例牌匾：弹出凡例内容（右墙，面板朝房间内 -x）
    if (hit.userData.kind === 'legend') {
      this.showMottoPanel(hit.userData.legends, hit.userData.style, wp, {
        offset: { x: -2.2, z: 0 },
        rotY: -Math.PI / 2,
      })
      return
    }
    this._pickCb(hit.userData.memberId, wp)
  }

  // 拾取面板（用于关闭）
  _pickPanel(e) {
    if (!this._panel || !this.renderer || !this.camera) return null
    const rect = this.renderer.domElement.getBoundingClientRect()
    const mouse = new THREE.Vector2(
      ((e.clientX - rect.left) / rect.width) * 2 - 1,
      -((e.clientY - rect.top) / rect.height) * 2 + 1
    )
    this._raycaster.setFromCamera(mouse, this.camera)
    const hits = this._raycaster.intersectObject(this._panel.group, true)
    return hits.length ? hits[0] : null
  }

  // 面板正面文字纹理
  _makePanelTexture(member) {
    const c = document.createElement('canvas')
    const W = 256, H = 288
    c.width = W
    c.height = H
    const ctx = c.getContext('2d')

    const grad = ctx.createLinearGradient(0, 0, W, 0)
    grad.addColorStop(0, '#5a4020')
    grad.addColorStop(0.5, '#543d24')
    grad.addColorStop(1, '#3f2d18')
    ctx.fillStyle = grad
    ctx.fillRect(0, 0, W, H)

    ctx.strokeStyle = '#c8a96a'
    ctx.lineWidth = 8
    ctx.strokeRect(14, 14, W - 28, H - 28)

    const name = (member.name || '').trim()
    ctx.fillStyle = '#f3e5c8'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.font = 'bold 36px "Songti SC", "SimSun", serif'
    ctx.fillText(name, W / 2, 56)

    ctx.strokeStyle = '#c8a96a'
    ctx.lineWidth = 2
    ctx.beginPath()
    ctx.moveTo(52, 92)
    ctx.lineTo(W - 52, 92)
    ctx.stroke()

    const rows = []
    if (member.generation) rows.push(`第${member.generation}世`)
    if (member.generation_word) rows.push(`字辈 ${member.generation_word}`)
    if (member.branch) rows.push(`房支 ${member.branch}`)
    const birth = member.birth_gregorian || member.birth_lunar || ''
    const death = member.death_gregorian || member.death_lunar || ''
    if (birth) rows.push(`生 ${birth}`)
    if (death) rows.push(`卒 ${death}`)
    if (member.lifespan) rows.push(`享年 ${member.lifespan} 岁`)

    ctx.fillStyle = '#dcc9a8'
    ctx.font = '21px "Songti SC", "SimSun", serif'
    const maxW = W - 56
    let y = 116
    for (const row of rows) {
      for (const line of wrapText(ctx, row, maxW)) {
        if (y > H - 20) break
        ctx.fillText(line, W / 2, y)
        y += 26
      }
    }

    const tex = new THREE.CanvasTexture(c)
    tex.colorSpace = THREE.SRGBColorSpace
    tex.anisotropy = 4
    return tex
  }

  // 构建 3D 面板（长方形木牌 + 厚度，前面贴内容，侧面/背面木纹，无底座）
  _buildPanel(member) {
    const W = 2.4, H = 2.8, D = 0.2
    const geo = new THREE.BoxGeometry(W, H, D)
    const woodMat = new THREE.MeshStandardMaterial({ color: '#4a3520', roughness: 0.6 })
    const panel = new THREE.Mesh(geo, woodMat)
    panel.castShadow = true

    const frontTex = this._makePanelTexture(member)
    const frontMat = new THREE.MeshStandardMaterial({ map: frontTex, roughness: 0.55 })
    const front = new THREE.Mesh(new THREE.PlaneGeometry(W * 0.92, H * 0.92), frontMat)
    front.position.z = D / 2 + 0.015
    panel.add(front)

    return panel
  }

  // 在指定世界坐标（牌位上方）显示 3D 面板
  showMemberPanel(member, worldPos) {
    this._clearPanel()
    const panel = this._buildPanel(member)
    panel.position.copy(worldPos)
    panel.position.y += 0.6
    panel.scale.setScalar(0.3)
    this.scene.add(panel)
    this._panel = {
      group: panel,
      fromY: worldPos.y + 0.6,
      targetY: worldPos.y + 2.0,
      t: 0,
    }
  }

  // 祖训/凡例大匾点击：在牌匾前面弹出内容面板
  showMottoPanel(mottoes, style, worldPos, opts = {}) {
    const { offset = { x: 0, z: 1.8 }, rotY = 0 } = opts
    this._clearPanel()
    const panel = this._buildMottoPanel(mottoes, style)
    panel.position.copy(worldPos)
    panel.position.x += offset.x
    panel.position.z += offset.z
    panel.rotation.y = rotY
    panel.scale.setScalar(0.3)
    this.scene.add(panel)
    this._panel = {
      group: panel,
      fromY: worldPos.y,
      targetY: worldPos.y + 0.8,
      t: 0,
    }
  }

  // 构建祖训内容面板
  _buildMottoPanel(mottoes, style) {
    const W = 5.2, H = 3.4, D = 0.08
    const geo = new THREE.BoxGeometry(W, H, D)
    const woodMat = new THREE.MeshStandardMaterial({ color: '#4a3520', roughness: 0.6 })
    const panel = new THREE.Mesh(geo, woodMat)
    panel.castShadow = true

    const frontTex = this._makeMottoPanelTexture(mottoes, style)
    const frontMat = new THREE.MeshStandardMaterial({ map: frontTex, roughness: 0.55 })
    const front = new THREE.Mesh(new THREE.PlaneGeometry(W * 0.92, H * 0.92), frontMat)
    front.position.z = D / 2 + 0.015
    panel.add(front)

    return panel
  }

  // 祖训内容面板纹理（横排，每条一行）
  _makeMottoPanelTexture(mottoes, style) {
    const c = document.createElement('canvas')
    const W = 1024, H = 670
    c.width = W
    c.height = H
    const ctx = c.getContext('2d')
    const st = this._getTotemStyle(style)

    const grad = ctx.createLinearGradient(0, 0, W, 0)
    grad.addColorStop(0, st.bg[0])
    grad.addColorStop(0.5, st.bg[1])
    grad.addColorStop(1, st.bg[2])
    ctx.fillStyle = grad
    ctx.fillRect(0, 0, W, H)
    ctx.strokeStyle = st.border
    ctx.lineWidth = 8
    ctx.strokeRect(10, 10, W - 20, H - 20)

    // 完整内容（每条 content 自动换行，不截断）
    const texts = (mottoes || [])
      .map((x) => (typeof x === 'string' ? x : (x && (x.content || x.title)) || ''))
      .map((t) => t.trim())
      .filter(Boolean)

    if (texts.length) {
      // 动态字号：完整放下所有文字
      let fs = 56
      let perLine = 1, lh = 1, totalLines = 0
      while (fs > 16) {
        perLine = Math.max(1, Math.floor((W - 160) / fs))
        lh = Math.floor(fs * 1.35)
        totalLines = texts.reduce((acc, t) => acc + Math.max(1, Math.ceil(Array.from(t).length / perLine)), 0)
        if (totalLines * lh <= H - 160) break
        fs -= 2
      }
      ctx.textAlign = 'center'
      ctx.textBaseline = 'middle'
      ctx.fillStyle = st.text
      ctx.font = `bold ${fs}px "Songti SC", "SimSun", serif`
      let y = (H - totalLines * lh) / 2 + lh / 2
      texts.forEach((t) => {
        const chars = Array.from(t)
        for (let i = 0; i < chars.length; i += perLine) {
          ctx.fillText(chars.slice(i, i + perLine).join(''), W / 2, y)
          y += lh
        }
      })
    }

    const tex = new THREE.CanvasTexture(c)
    tex.colorSpace = THREE.SRGBColorSpace
    tex.anisotropy = 4
    return tex
  }

  hideMemberPanel() {
    this._clearPanel()
  }

  _clearPanel() {
    if (!this._panel) return
    const g = this._panel.group
    if (g.parent) g.parent.remove(g)
    g.traverse((o) => {
      if (o.geometry) o.geometry.dispose()
      if (o.material) {
        const mats = Array.isArray(o.material) ? o.material : [o.material]
        mats.forEach((m) => {
          if (m.map) m.map.dispose()
          m.dispose()
        })
      }
    })
    this._panel = null
  }

  _onMove = (e) => {
    if (this._viewMode === 'free') {
      if (this._pointerLocked) {
        // 已锁定：用 movementX/Y 转视角（准星模式），指针隐藏
        this._freeYaw -= e.movementX * 0.003
        this._freePitch = Math.max(-1.4, Math.min(1.4, this._freePitch - e.movementY * 0.003))
        return
      }
      if (this._dragging && this._lastPointer) {
        // 未锁定：左键拖拽转视角
        const dx = e.clientX - this._lastPointer.x
        const dy = e.clientY - this._lastPointer.y
        this._lastPointer = { x: e.clientX, y: e.clientY }
        this._freeYaw -= dx * 0.005
        this._freePitch = Math.max(-1.4, Math.min(1.4, this._freePitch - dy * 0.005))
      }
    } else if (this._viewMode === 'first') {
      if (this._pointerLocked) {
        // 已锁定：用 movementX/Y 转视角（FPS）
        this._camYaw -= e.movementX * 0.003
        this._camPitch = Math.max(-1.2, Math.min(1.2, this._camPitch - e.movementY * 0.003))
        return
      }
      if (this._dragging && this._lastPointer) {
        const dx = e.clientX - this._lastPointer.x
        const dy = e.clientY - this._lastPointer.y
        this._lastPointer = { x: e.clientX, y: e.clientY }
        this._camYaw -= dx * 0.005
        this._camPitch = Math.max(-1.2, Math.min(1.2, this._camPitch - dy * 0.005))
      }
    }
    const hit = this._pickAt(e)
    const id = hit ? (hit.userData.memberId || hit.userData.kind) : null
    if (id !== this._hoverId) {
      this._hoverId = id
      this.renderer.domElement.style.cursor = id != null ? 'pointer' : 'default'
    }
  }

  _pickAt(e) {
    if (!this.renderer || !this.camera) return null
    const rect = this.renderer.domElement.getBoundingClientRect()
    const mouse = new THREE.Vector2(
      ((e.clientX - rect.left) / rect.width) * 2 - 1,
      -((e.clientY - rect.top) / rect.height) * 2 + 1
    )
    this._raycaster.setFromCamera(mouse, this.camera)
    const objs = this._tabletGroup.children.concat(this._plaqueMeshes)
    const hits = this._raycaster.intersectObjects(objs, true)
    return hits.length ? hits[0].object : null
  }

  _pickAtCenter() {
    if (!this.renderer || !this.camera) return null
    const mouse = new THREE.Vector2(0, 0)  // 屏幕中心 NDC（准星位置）
    this._raycaster.setFromCamera(mouse, this.camera)
    const objs = this._tabletGroup.children.concat(this._plaqueMeshes)
    const hits = this._raycaster.intersectObjects(objs, true)
    return hits.length ? hits[0].object : null
  }

  onPick(cb) {
    this._pickCb = cb
  }

  highlight(memberId) {
    this._highlightId = memberId
    this._tabletGroup.children.forEach((t) => {
      if (t.userData.memberId === memberId) {
        t.scale.set(1.08, 1.08, 1.08)
      } else if (this._highlightId == null) {
        t.scale.set(1, 1, 1)
      } else {
        t.scale.set(1, 1, 1)
      }
    })
  }

  setWatermark(show) {
    if (!this._watermarkEl) return
    const level = getWatermarkLevel()
    if (!show || level <= 0) {
      this._watermarkEl.style.display = 'none'
      this._watermarkEl.innerHTML = ''
      return
    }
    const text = '试用版·来源系统'
    const frag = document.createDocumentFragment()
    const n = 24
    for (let i = 0; i < n; i++) {
      const span = document.createElement('span')
      span.textContent = text
      Object.assign(span.style, {
        position: 'absolute',
        left: `${(i % 6) * 18}%`,
        top: `${(i * 17) % 90}%`,
        transform: 'rotate(-28deg)',
        fontSize: '20px',
        color: '#999',
        opacity: level >= 3 ? '0.9' : level >= 2 ? '0.6' : '0.35',
        whiteSpace: 'nowrap',
        fontFamily: 'sans-serif',
      })
      frag.appendChild(span)
    }
    this._watermarkEl.style.display = 'block'
    this._watermarkEl.appendChild(frag)
  }

  async capture() {
    throw new Error('capture() 未实现')
  }

  _onResize = () => {
    if (!this.container || !this.camera || !this.renderer) return
    const w = this.container.clientWidth
    const h = this.container.clientHeight
    this.camera.aspect = w / h
    this.camera.updateProjectionMatrix()
    this.renderer.setSize(w, h)
  }

  _animate = () => {
    this._raf = requestAnimationFrame(this._animate)
    const now = performance.now()
    const dt = this._lastT ? Math.min((now - this._lastT) / 1000, 0.1) : 0.016
    this._lastT = now
    this._updateIncense(dt)
    this._updateSmoke(dt)
    if (this._viewMode === 'first' || this._viewMode === 'free') {
      this._updateMove()
      this._updateCamera()
    } else if (this._controls) {
      this._controls.update()
    }
    if (this._panel) {
      this._panel.t = Math.min(this._panel.t + 0.05, 1)
      const e = 1 - Math.pow(1 - this._panel.t, 3)
      const g = this._panel.group
      g.position.y = this._panel.fromY + (this._panel.targetY - this._panel.fromY) * e
      g.scale.setScalar(0.3 + (1 - 0.3) * e)
      // 第一/自由视角：面板水平朝向自己当前位置（不倾斜，只绕 y 轴）
      if (this._viewMode === 'first' || this._viewMode === 'free') {
        const dx = this.camera.position.x - g.position.x
        const dz = this.camera.position.z - g.position.z
        g.rotation.y = Math.atan2(dx, dz)
      }
    }
    if (this.renderer) this.renderer.render(this.scene, this.camera)
  }

  dispose() {
    cancelAnimationFrame(this._raf)
    window.removeEventListener('resize', this._onResize)
    if (this.renderer) {
      const el = this.renderer.domElement
      el.removeEventListener('wheel', this._onWheel)
      el.removeEventListener('mousedown', this._onMouseDown)
      el.removeEventListener('click', this._onClick)
      el.removeEventListener('mousemove', this._onMove)
      el.removeEventListener('mouseup', this._onMouseUp)
      el.removeEventListener('mouseleave', this._onMouseUp)
      el.removeEventListener('contextmenu', this._onContextMenu)
      el.removeEventListener('touchstart', this._onTouchStart)
      el.removeEventListener('touchmove', this._onTouchMove)
      el.removeEventListener('touchend', this._onTouchEnd)
      window.removeEventListener('keydown', this._onKeyDown)
      window.removeEventListener('keyup', this._onKeyUp)
      document.removeEventListener('pointerlockchange', this._onPointerLockChange)
      this.renderer.dispose()
      if (el.parentNode) el.parentNode.removeChild(el)
    }
    if (this._controls) this._controls.dispose()
    if (this._joystick && this._joystick.el.parentNode) this._joystick.el.parentNode.removeChild(this._joystick.el)
    if (this._watermarkEl && this._watermarkEl.parentNode) this._watermarkEl.parentNode.removeChild(this._watermarkEl)
    if (this._crosshairEl && this._crosshairEl.parentNode) this._crosshairEl.parentNode.removeChild(this._crosshairEl)
    this._clearPanel()
    this._clearHall()
    this.scene = null
    this.renderer = null
    this._controls = null
    this._joystick = null
  }
}
