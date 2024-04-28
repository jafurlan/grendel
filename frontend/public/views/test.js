import * as THREE from "/static/threejs/three.module.min.js";
import { OrbitControls } from "/static/threejs/addons/OrbitControls.js";
import { CSS2DRenderer, CSS2DObject } from "/static/threejs/addons/CSS2DRenderer.js";

const container = document.querySelector("[data-renderer]");

const renderer = new THREE.WebGLRenderer();
renderer.setSize(container.clientWidth, container.clientHeight);
container.appendChild(renderer.domElement);

const labelRenderer = new CSS2DRenderer();
labelRenderer.setSize(container.clientWidth, container.clientHeight);
labelRenderer.domElement.style.position = "absolute";
labelRenderer.domElement.style.top = "0px";
labelRenderer.domElement.style.pointerEvents = "none";
container.appendChild(labelRenderer.domElement);

const scene = new THREE.Scene();
const camera = new THREE.PerspectiveCamera(45, container.clientWidth / container.clientHeight, 1, 1000);
camera.position.set(10, 15, -22);

const orbit = new OrbitControls(camera, renderer.domElement);
orbit.update();

const planeMesh = new THREE.Mesh(
    new THREE.PlaneGeometry(20, 20),
    new THREE.MeshBasicMaterial({
        side: THREE.DoubleSide,
        visible: false,
    }),
);

planeMesh.rotateX(-Math.PI / 2);
scene.add(planeMesh);

const grid = new THREE.GridHelper(20, 20);
scene.add(grid);

const highlightMesh = new THREE.Mesh(
    new THREE.PlaneGeometry(1, 1),
    new THREE.MeshBasicMaterial({
        side: THREE.DoubleSide,
        transparent: true,
    }),
);
highlightMesh.rotateX(-Math.PI / 2);
highlightMesh.position.set(0.5, 0, 0.5);
scene.add(highlightMesh);

const mousePosition = new THREE.Vector2();
const raycaster = new THREE.Raycaster();
let intersects;

window.addEventListener("mousemove", function (e) {
    mousePosition.x = (e.offsetX / container.clientWidth) * 2 - 1;
    mousePosition.y = -(e.offsetY / container.clientHeight) * 2 + 1;
    raycaster.setFromCamera(mousePosition, camera);
    intersects = raycaster.intersectObject(planeMesh);
    if (intersects.length > 0) {
        const intersect = intersects[0];
        const highlightPos = new THREE.Vector3().copy(intersect.point).floor().addScalar(0.5);
        highlightMesh.position.set(highlightPos.x, 0, highlightPos.z);

        const objectExist = objects.find(function (object) {
            return object.position.x === highlightMesh.position.x && object.position.z === highlightMesh.position.z;
        });

        if (!objectExist) highlightMesh.material.color.setHex(0xffffff);
        else highlightMesh.material.color.setHex(0xff0000);
    }
});

const rackMesh = new THREE.Mesh(
    new THREE.BoxGeometry(1, 2, 1).translate(0, 1, 0),
    new THREE.MeshBasicMaterial({
        wireframe: true,
        color: 0xffea00,
    }),
);

const rackDiv = document.createElement("div");
rackDiv.className = "label";
rackDiv.style.backgroundColor = "transparent";
rackDiv.style.color = "white";
const rackLabel = new CSS2DObject(rackDiv);
rackLabel.position.set(0, 0, 0);
rackLabel.center.set(0.5, 0);
rackMesh.add(rackLabel);
rackLabel.layers.set(0);

const objects = [];

window.addEventListener("mousedown", function () {
    const objectExist = objects.find(function (object) {
        return object.position.x === highlightMesh.position.x && object.position.z === highlightMesh.position.z;
    });

    if (!objectExist) {
        if (intersects.length > 0) {
            rackDiv.textContent = `rack ${objects.length + 1}`;
            // rackDiv.textContent = `rack ${highlightMesh.position.x}, ${highlightMesh.position.z}`;

            const rackClone = rackMesh.clone();
            rackMesh.rackClone.position.copy(highlightMesh.position);
            scene.add(rackClone);
            objects.push(rackClone);
            highlightMesh.material.color.setHex(0xff0000);
        }
    }
    console.log(scene.children.length);
});

function animate(time) {
    highlightMesh.material.opacity = 1 + Math.sin(time / 120);
    renderer.render(scene, camera);
    labelRenderer.render(scene, camera);
}
renderer.setAnimationLoop(animate);

window.addEventListener("resize", function () {
    camera.aspect = container.clientWidth / container.clientHeight;
    camera.updateProjectionMatrix();
    renderer.setSize(container.clientWidth, container.clientHeight);
    labelRenderer.setSize(container.clientWidth, container.clientHeight);
});
