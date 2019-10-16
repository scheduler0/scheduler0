// @ts-ignore
import express from 'express';
import axiosInstance from '../misc/axiosInstance'

const router = express.Router();

router.route("/")
    .post((req, res) => {
        axiosInstance
            .post('/credentials', req.body)
            .then((resp) => {
                res.status(resp.status).send(resp.data);
            }).catch((e) => {
                res.status(e.response.status).send(e.response.data);
            });
    })
    .get(async (_, res) => {
        let resp;

        try {
            resp = await axiosInstance.get('/credentials');
            res.status(resp.status).send(resp.data);
        } catch (e) {
            res.status(e.status).send(e.data);
        }
    });

router.route("/:id")
    .delete(async (req, res) => {
        axiosInstance.delete(`/credentials/${req.params.id}`)
            .then((resp) => {
                res.status(resp.status).send(resp.data);
            }).catch((e) => {
                res.status(e.response.status).send(e.response.data);
            });
    })
    .get(async (req, res) => {
        await axiosInstance.get(`/credentials/${req.params.id}`)
            .then((resp) => {
                res.status(resp.status).send(resp.data);
            }).catch((e) => {
                res.status(e.response.status).send(e.response.data);
            });
    })
    .put(async (req, res) => {
        await axiosInstance.put(`/credentials/${req.params.id}`, req.body)
            .then((resp) => {
                res.status(resp.status).send(resp.data);
            }).catch((e) => {
                res.status(e.response.status).send(e.response.data);
            });
    });

export default router;